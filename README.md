
# mdremind

mdremind is a simple reminder tool that watches your notes directory for markdown files containing reminder entries and sends notifications based on their due dates.

## Installation
```bash
go install github.com/AyushRawal/mdremind
```

## Usage

- Ensure your reminder entries in Markdown files follow the following format ([dataview](https://blacksmithgu.github.io/obsidian-dataview/) inline metadata format):
```markdown
- [ ] <title> [due:: <datetime>]
```
- Add the configuration file.
On Linux, it looks for the configration file at `$XDG_CONFIG_HOME/mdremind.jsonc` if `$XDG_CONFIG_HOME` is set, else `$HOME/.config/mdremind.jsonc`. On Windows, it looks for `%AppData%\mdremind.jsonc`.

- Run mdremind.

## Configuration

Sample configuration (for linux based OS):

```jsonc
// mdremind.jsonc
{
    "notes_directory_path": "${HOME}/Notes",
    "default_reminder_time": "09:00 AM",
    "notification_cmd": "notify-send",
    "notification_cmd_arguments": [
        "-i",
        "calendar",
        "Reminder"
    ],
    "reminder_datetime_format": "2006-01-02 3:04 PM", // go datetime format, see: `https://go.dev/src/time/format.go`
    // "timezone": "", // optional, uses system's timezone by default (except for android, see: `https://github.com/golang/go/issues/20455`)
    "ignored_directories": [ // optional
        ".git",
        ".obsidian",
        ".trash",
        "templates",
        ".stfolder",
        "Excalidraw",
        "assets"
    ]
}
```

## Contributing

Contributions are welcome! If you have any ideas for improvements, feature requests, or bug reports, please open an issue or submit a pull request.

## License

This project is licensed under the MIT License.
