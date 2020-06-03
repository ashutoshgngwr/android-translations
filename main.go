package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

// xmlStringResources declares data structure for unmarshalling 'resources' tag in
// Android values XML files.
type xmlStringResources struct {
	xml.Name `xml:"resources"`
	Strings  []xmlStringResource `xml:"string"`
}

// xmlStringResource declares data structure for unmarshalling 'string' tags in Android
// values XML files.
type xmlStringResource struct {
	Name         string `xml:"name,attr"`
	Value        string `xml:",chardata"`
	Translatable string `xml:"translatable,attr"`
	Locale       string `xml:"-"`
}

// localeStringsMap declares the type to map locales => string_name => stringResource
type localeStringsMap map[string]map[string]xmlStringResource

// stringResource declares the output structure for a single string resource.
type stringResource struct {
	Name           string   `json:"name"`
	Value          string   `json:"value"`
	MissingLocales []string `json:"missing_locales"`
}

// MissingLocalesString joins the MissingLocales slice using ", " separator
func (res stringResource) MissingLocalesString() string {
	return strings.Join(res.MissingLocales, ", ")
}

// defaultLocale declares the constant to identify default string resources (resources
// in 'values' [no suffix] directory)
const defaultLocale = "default"

var (
	projectDir    string // root directory of the Android Project
	outputFormat  string // output format, must be one of markdown or json
	markdownTitle string // heading for markdown content
	githubActions bool   // if true, also call setGitHubActionsOutput to set action output
)

func init() {
	pflag.CommandLine.SortFlags = false
	pflag.StringVar(&projectDir, "project-dir", ".", "Android Project's root directory")
	pflag.StringVar(&outputFormat, "output-format", "json", "Output format. Must be 'json' or 'markdown'")
	pflag.StringVar(&markdownTitle, "markdown-title", "Missing Translations", "Title for the Markdown content")
	pflag.BoolVar(&githubActions, "github-actions", false, "Indicates if the runtime is GitHub Actions")
	pflag.Parse()

	if outputFormat != "json" && outputFormat != "markdown" {
		fatal(fmt.Sprintf("unknow output format %s", outputFormat))
	}
}

func main() {
	valuesFiles, err := findValuesFiles(projectDir)
	if err != nil {
		fatal(err)
	}

	localeStrings, err := findTranslatableStrings(valuesFiles)
	if err != nil {
		fatal(err)
	}

	defaultStrings, ok := localeStrings[defaultLocale]
	if !ok { // shouldn't be true for valid input
		fatal("unable to find string resources for default locale")
	}

	missingTranslations := make([]stringResource, 0)
	for _, str := range defaultStrings {
		strResource := stringResource{
			Name:           str.Name,
			Value:          str.Value,
			MissingLocales: make([]string, 0),
		}

		for locale := range localeStrings {
			if _, ok := localeStrings[locale][str.Name]; !ok {
				strResource.MissingLocales = append(strResource.MissingLocales, locale)
			}
		}

		if len(strResource.MissingLocales) > 0 {
			missingTranslations = append(missingTranslations, strResource)
		}
	}

	var output string
	switch outputFormat {
	case "json":
		output = mustRenderJSON(missingTranslations)
		break
	case "markdown":
		output = mustRenderMarkdown(markdownTitle, missingTranslations)
		break
	}

	if githubActions {
		setGitHubActionsOutput("report", output)
		fmt.Println()
	}

	fmt.Println(output)
}

// fatal is a convenience function that calls 'fmt.Println' with 'msg' followed by an
// 'os.Exit(1)' invocation.
func fatal(msg interface{}) {
	fmt.Println("error:", msg)
	os.Exit(1)
}

// findValuesFiles finds XML files in 'path/**/*/values*'. This function should be
// compatible with cases where multiple resource directories are in use.
func findValuesFiles(path string) ([]string, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read directory %s", path)
	}

	valuesFiles := make([]string, 0)
	for _, file := range files {
		filePath := filepath.Join(path, file.Name())
		if isGitIgnored(path, filePath) {
			continue
		}

		if file.IsDir() {
			moreValuesFiles, err := findValuesFiles(filePath)
			if err != nil {
				return nil, err
			}

			valuesFiles = append(valuesFiles, moreValuesFiles...)
		} else {
			if isValuesFile(filePath) {
				valuesFiles = append(valuesFiles, filePath)
			}
		}
	}

	return valuesFiles, nil
}

func isValuesFile(path string) bool {
	parent := filepath.Base(filepath.Dir(path))
	return strings.HasPrefix(parent, "values") && strings.EqualFold(".xml", filepath.Ext(path))
}

// findTranslatableStrings looks for '<string>' tags with '<resources>' tag as its root
// in given files. It parses all the string tags without 'translatable="fasle"' attribute.
// It returns a mapping of locale to their strings where locale is suffix of 'values-'.
// If no suffix is present, i.e. 'values', defaultLocale constant is used to identify those
// values.
func findTranslatableStrings(files []string) (localeStringsMap, error) {
	strResources := make(localeStringsMap, 0)
	for _, file := range files {
		content, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to read file at %s", file)
		}

		resources := &xmlStringResources{}
		err = xml.Unmarshal(content, resources)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to parse XML file at %s", file)
		}

		locale := getLocaleForValuesFile(file)
		for _, str := range resources.Strings {
			if !strings.EqualFold(str.Translatable, "false") {
				if _, ok := strResources[locale]; !ok {
					strResources[locale] = map[string]xmlStringResource{}
				}

				strResources[locale][str.Name] = str
			}
		}
	}

	return strResources, nil
}

// getLocaleForValuesFile returns the suffix after 'values-'. If no suffix is present,
// e.g. 'values', it returns the defaultLocale constant.
func getLocaleForValuesFile(path string) string {
	parent := filepath.Base(filepath.Dir(path))
	if strings.EqualFold(parent, "values") {
		return defaultLocale
	}

	split := strings.SplitN(parent, "-", 2)
	if len(split) < 2 { // edge case. shouldn't be true for valid input
		return defaultLocale
	}

	return split[1]
}

// isGitIgnored checks if the given path is ignored from being tracked by 'git'. 'workingDir'
// is used provide additional to 'git' command. It returns false, if 'workingDir' is not an
// ancestor of the given file path.
func isGitIgnored(workingDir, file string) bool {
	relFilePath, err := filepath.Rel(workingDir, file)
	if err != nil {
		return false
	}

	cmd := exec.Command("git", "check-ignore", relFilePath)
	cmd.Dir = workingDir
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

func mustRenderJSON(v interface{}) string {
	content, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(errors.Wrap(err, "failed to marshal content as JSON"))
	}

	return string(content)
}

func mustRenderMarkdown(title string, data []stringResource) string {
	mdTemplate, err := template.New("markdown").Parse(`# {{ .title }}

{{ if eq .length 0 -}}
No missing translations found.
{{ else -}}
{{ .table }}
{{- end }}
_Generated using [Android Missing Translations][1] GitHub action._

[1]: https://github.com/ashutoshgngwr/android-missing-translations
`)

	var content bytes.Buffer
	err = mdTemplate.Execute(&content, map[string]interface{}{
		"title":  title,
		"length": len(data),
		"table":  renderMarkdownTable(data),
	})

	if err != nil {
		panic(errors.Wrap(err, "unable to render data as markdown"))
	}

	return content.String()
}

func renderMarkdownTable(data []stringResource) string {
	var tableContent bytes.Buffer
	table := tablewriter.NewWriter(&tableContent)
	table.SetBorders(tablewriter.Border{Left: true, Right: true})
	table.SetCenterSeparator("|")
	table.SetHeader([]string{"#", "Name", "Default Value", "Missing Locales"})
	for i, item := range data {
		table.Append(
			[]string{
				fmt.Sprintf("%d", 1+i),
				fmt.Sprintf("`%s`", item.Name),
				item.Value,
				item.MissingLocalesString(),
			},
		)
	}

	table.Render()
	return tableContent.String()
}

func setGitHubActionsOutput(key, value string) {
	value = strings.ReplaceAll(value, "%", "%25")
	value = strings.ReplaceAll(value, "\r", "%0D")
	value = strings.ReplaceAll(value, "\n", "%0A")
	value = strings.ReplaceAll(value, ":", "%3A")
	value = strings.ReplaceAll(value, ",", "%2C")
	fmt.Printf("::set-output name=%s::%s\n", key, value)
}
