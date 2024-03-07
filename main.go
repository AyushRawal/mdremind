package main

import (
	"io/fs"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"muzzammil.xyz/jsonc"
)

type Reminder struct {
	Title string
	Time  time.Time
}

type Config struct {
	NotesDir                 string   `json:"notes_directory_path"`
	DefaultReminderTimeOfDay string   `json:"default_reminder_time"`
	NotifyCmd                string   `json:"notification_cmd"`
	NotifyCmdArgs            []string `json:"notification_cmd_arguments,omitempty"`
	ReminderDateTimeFormat   string   `json:"reminder_datetime_format"`
	TimeZoneLocation         string   `json:"timezone,omitempty"`
	IgnoredDirs              []string `json:"ignored_directories,omitempty"`
	TimeZone                 *time.Location
}

var (
	config    Config
	reminders []Reminder
)

func readConfig(filePath string) error {
	fileContents, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	if err := jsonc.Unmarshal(fileContents, &config); err != nil {
		return err
	}
	config.NotesDir = os.ExpandEnv(config.NotesDir)
	config.TimeZone = time.Local
	if config.TimeZoneLocation != "" {
		config.TimeZone, err = time.LoadLocation(config.TimeZoneLocation)
		if err != nil {
			return err
		}
	}
	return nil
}

func parseReminderEntries(fileContents []byte) []Reminder {
	reg := regexp.MustCompile(`(?m)^ *- \[ \] *(?P<title>.*) \[due:: (?P<remind>.*?)\].*$`)
	matches := reg.FindAllSubmatch(fileContents, -1)
	var reminders []Reminder
	for _, m := range matches {
		title := string(m[1])
		datetime := strings.TrimSpace(string(m[2]))
		if len(strings.Split(datetime, " ")) == 1 {
			datetime += " " + config.DefaultReminderTimeOfDay
		}
		datetimeParsed, err := time.ParseInLocation(config.ReminderDateTimeFormat, datetime, config.TimeZone)
		if err != nil {
			log.Printf("[WARN] Invalid datetime %s: %s", datetime, err)
			continue
		}
		// log.Printf("[INFO] Reminder: %s <%s>", title, datetimeParsed.String())
		reminders = append(reminders, Reminder{
			Title: title,
			Time:  datetimeParsed,
		})
	}
	return reminders
}

func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func findMarkdownFiles(dirPath string) ([]string, error) {
	var markdownFiles []string
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if contains(config.IgnoredDirs, d.Name()) {
				return filepath.SkipDir // Skip this directory
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".md") {
			markdownFiles = append(markdownFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return markdownFiles, nil
}

func notify(msg string) {
	args := append(config.NotifyCmdArgs, msg)
	// log.Printf("[INFO] Executing command: %s %s", config.NotifyCmd, strings.Join(args, " "))
	if err := exec.Command(config.NotifyCmd, args...).Run(); err != nil {
		log.Printf("[ERROR] Could not execute command '%s %s': %s", config.NotifyCmd, strings.Join(args, " "), err)
	}
}

func remindLoop() {
	for _, reminder := range reminders {
		if reminder.Time.Compare(time.Now()) <= 0 {
			notify(reminder.Title)
		}
	}
	for range time.Tick(1 * time.Minute) {
		for _, reminder := range reminders {
			if reminder.Time.Format(config.ReminderDateTimeFormat) == time.Now().In(config.TimeZone).Format(config.ReminderDateTimeFormat) {
				notify(reminder.Title)
			}
		}
	}
}

func watchDirRecursive(watcher *fsnotify.Watcher, dirPath string) {
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && !contains(config.IgnoredDirs, d.Name()) {
			err := watcher.Add(path)
			if err != nil {
				log.Printf("[ERROR] Failed to watch directory %s: %s", path, err)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("[ERROR] Failed to initialize directory watch: %s", err)
	}
}

func loadReminders(notesDir string) []Reminder {
	reminders := []Reminder{}
	markdownFiles, err := findMarkdownFiles(notesDir)
	if err != nil {
		log.Fatalf("[ERROR] Error finding markdown files: %s", err)
	}
	// log.Printf("[INFO] Found %d markdown files", len(markdownFiles))
	for _, f := range markdownFiles {
		fileContents, err := os.ReadFile(f)
		if err != nil {
			log.Printf("[ERROR] Could not read file %s: %s", f, err)
			continue
		}
		reminders = append(reminders, parseReminderEntries(fileContents)...)
	}
	return reminders
}

func main() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("[ERROR] Could not determine config directory: %s", err)
	}
	if err = readConfig(filepath.Join(configDir, "mdremind.jsonc")); err != nil {
		log.Fatalf("[ERROR] Could not read config: %s", err)
	}
	reminders = loadReminders(config.NotesDir)
	go remindLoop()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("[ERROR] Could not create watcher: %s", err)
	}
	defer watcher.Close()
	watchDirRecursive(watcher, config.NotesDir)

	var eventTimers = make(map[string]*time.Timer)
	var timersMutex sync.Mutex

	for {
		select {
		case fsEvent, ok := <-watcher.Events:
			if !ok {
				return
			}
			// log.Printf("[INFO] Watcher event: %s", fsEvent.String())
			if fsEvent.Has(fsnotify.Create) {
				info, err := os.Stat(fsEvent.Name)
				if err == nil && info.IsDir() {
					watcher.Add(fsEvent.Name)
					continue
				}
			}
			if filepath.Ext(fsEvent.Name) != ".md" {
				continue
			}
			if fsEvent.Has(fsnotify.Remove) {
				reminders = loadReminders(config.NotesDir)
			}
			if fsEvent.Has(fsnotify.Create) || fsEvent.Has(fsnotify.Write) {
				timersMutex.Lock()
				timer, ok := eventTimers[fsEvent.Name]
				timersMutex.Unlock()
				if !ok {
					timer = time.AfterFunc(math.MaxInt64, func() {
						reminders = loadReminders(config.NotesDir)
					})
					timer.Stop()
					timersMutex.Lock()
					eventTimers[fsEvent.Name] = timer
					timersMutex.Unlock()
				}
				timer.Reset(100 * time.Millisecond)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[ERROR] From watcher: %s", err)
		}
	}
}
