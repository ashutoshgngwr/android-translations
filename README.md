# Android Translations

![Docker](https://github.com/ashutoshgngwr/android-translations/workflows/Docker/badge.svg)
![Docker image size](https://img.shields.io/docker/image-size/ashutoshgngwr/android-translations?sort=semver)

A GitHub Action to find the missing and _potentially_ outdated translations
for existing locales in an Android Project.

This action is the same as running the following but with a few exceptions

1. Reports can be generated in JSON. Lint tool generates XML or HTML
2. Seamless integration with GitHub actions to ease the use of generated
   data
3. It's not easy to [run the standalone lint tool on a Gradle project
   ](https://stackoverflow.com/q/62149318/2410641)
4. It checks Git blame to find outdated translations

```sh
${ANDROID_HOME}/tools/bin/lint --check MissingTranslation ${PROJECT_DIR}
```

## Features

- Almost zero config
- Find outdated translations
- Generate reports in Markdown or JSON format
- Usable in other CI environments

## Usage

The following workflow covers one possible use-case for this action.
It uses the action to find the missing translations and generate a report
in Markdown format. In the next step, it creates a comment on issue #1
with the generated report as its body.

```yaml
on: [push]

jobs:
  check-translations:
    name: Check Translations
    # Linux is required since this is a docker container action
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - id: check_translations
        uses: ashutoshgngwr/android-translations@v1
        with:
          projectDir: ./
      - name: Add comment
        uses: peter-evans/create-or-update-comment@v1
        with:
          issue-number: 1
          body: ${{ steps.check_translations.outputs.report }}
```

### Input

The action can accept the following input parameters

| Key               | Description                                          | Default Value          |
| ----------------- | ---------------------------------------------------- | ---------------------- |
| `projectDir`      | Android Project's root directory                     | `.`                    |
| `outdatedLocales` | If true, also find potentially outdated translations | `true`                 |
| `outputFormat`    | Must be one of `json` or `markdown`                  | `markdown`             |
| `markdownTitle`   | Title for the Markdown content (not used with JSON)  | `Missing Translations` |

### Output

The action produces the following output which can be used in the next steps
or jobs. See [`steps` context](https://help.github.com/en/actions/reference/context-and-expression-syntax-for-github-actions#needs-context)
and [`needs` context](https://help.github.com/en/actions/reference/context-and-expression-syntax-for-github-actions#needs-context).
In addition to this, the action also prints the same output to `stdout`.

| Key      | Description                                                          |
| -------- | -------------------------------------------------------------------- |
| `report` | The missing translations report for strings in the requested format. |

#### JSON Report Format

The following structure is used while generating JSON reports.

```json
[
  {
    "name": "example",
    "value": "This is an example",
    "missing_locales": [
      "hi",
      "ru-RU"
    ],
    "outdated_locales": [
      "de"
    ]
  },
  "...more of the same stuff..."
]
```

### Using Without GitHub Actions

**Caution:** The action is designed to run on projects that are part of a Git repository.

```sh
docker run --rm --workdir /app --mount type=bind,source="$(pwd)",target=/app \
   ashutoshgngwr/android-translations:v1 --output-format=json
```

## License

[Apache License 2.0](/LICENSE)
