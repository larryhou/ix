package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"encoding/json"
	"github.com/larryhou/ix/api/afc"
	"github.com/larryhou/ix/api/device"
	"github.com/larryhou/ix/api/dvt/processctrl"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

func init() {
	log.SetFlags(log.LstdFlags)
}

// usage prints top-level help.
func usage() {
	fmt.Fprintf(os.Stderr, `Usage: devicetool <command> [flags]

Commands:
  snap        take a screenshot
  launch      launch an app
  kill        kill a running app
  install     install an IPA
  uninstall   uninstall an app
  pull        pull file(s) from app sandbox
  push        push file(s) into app sandbox
  remove      remove a file from app sandbox
  process     manage running processes (-list / -kill <pid> / -kill-bundle <bundle> / -relaunch <bundle>)
  list        list installed apps
  log         stream syslog
  unzip       unzip a local file (no device needed)
  fs          sandbox filesystem: ls / du / clean

Use "devicetool <command> -help" for command-specific flags.
`)
	os.Exit(1)
}

// newFlagSet creates a FlagSet with -udid pre-registered.
func newFlagSet(name string) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	udid := fs.String(`udid`, ``, `device UDID (default: auto-select)`)
	return fs, udid
}

func openDevice(udid string) *device.Service {
	dev, err := device.New(udid)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return dev
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// --- file helpers ---

func getAllFiles(dirPth, dirName string) ([]string, error) {
	fis, err := os.ReadDir(filepath.Clean(filepath.ToSlash(dirPth)))
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range fis {
		rel := filepath.Join(dirName, f.Name())
		if f.IsDir() {
			sub, err := getAllFiles(filepath.Join(dirPth, f.Name()), rel)
			if err != nil {
				return nil, err
			}
			files = append(files, sub...)
		} else {
			files = append(files, rel)
		}
	}
	return files, nil
}

func unzipFile(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	if err = os.MkdirAll(dest, 0777); err != nil {
		return err
	}
	for _, file := range r.File {
		fPath := filepath.Join(dest, file.Name)
		if file.FileInfo().IsDir() {
			if err = os.MkdirAll(fPath, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(fPath), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(fPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func pullFile(afcSvc *afc.Service, remoteFile, localFile string) error {
	h, err := afcSvc.Open(remoteFile, `r`)
	if err != nil {
		return fmt.Errorf("open remote %s: %w", remoteFile, err)
	}
	defer h.Close()
	r, err := h.FileReader()
	if err != nil {
		return fmt.Errorf("reader %s: %w", remoteFile, err)
	}
	if err = os.MkdirAll(filepath.Dir(localFile), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(localFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = io.Copy(f, r); err != nil {
		return fmt.Errorf("copy %s: %w", remoteFile, err)
	}
	log.Printf(`pull %s -> %s`, remoteFile, localFile)
	return nil
}

func pushFile(afcSvc *afc.Service, localFile, remotePath string) error {
	info, err := os.Stat(localFile)
	if err != nil {
		return err
	}
	h, err := afcSvc.Open(remotePath, `w`)
	if err != nil {
		return fmt.Errorf("open remote %s: %w", remotePath, err)
	}
	defer h.Close()
	w, err := h.FileWriter(info.Size())
	if err != nil {
		return fmt.Errorf("writer %s: %w", remotePath, err)
	}
	f, err := os.Open(localFile)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = io.Copy(w, f); err != nil {
		return fmt.Errorf("copy %s: %w", localFile, err)
	}
	log.Printf(`push %s -> %s`, localFile, remotePath)
	return nil
}

func pushDir(afcSvc *afc.Service, localPath, remotePath string) error {
	files, err := getAllFiles(localPath, "")
	if err != nil {
		return err
	}
	if err = afcSvc.MkDir(remotePath); err != nil {
		return err
	}
	for _, rel := range files {
		if dir := filepath.Dir(rel); dir != "." {
			if err = afcSvc.MkDir(filepath.Join(remotePath, dir)); err != nil {
				return err
			}
		}
		if err = pushFile(afcSvc, filepath.Join(localPath, rel), filepath.Join(remotePath, rel)); err != nil {
			return err
		}
	}
	return nil
}

// --- subcommands ---

func runSnap(args []string) {
	fs, udid := newFlagSet(`snap`)
	out := fs.String(`out`, `/tmp/screenshot.png`, `output file path`)
	fs.Parse(args)

	dev := openDevice(*udid)
	data, err := dev.ScreenShot()
	fatal(err)
	fatal(os.WriteFile(*out, data, 0644))
	log.Printf(`screenshot saved to %s (%d bytes)`, *out, len(data))
}

func runLaunch(args []string) {
	fs, udid := newFlagSet(`launch`)
	bundle := fs.String(`bundle`, ``, `bundle identifier (required)`)
	fs.Parse(args)
	if *bundle == `` {
		fmt.Fprintln(os.Stderr, "launch: -bundle is required")
		fs.Usage()
		os.Exit(1)
	}

	dev := openDevice(*udid)
	pid, err := dev.Launch(*bundle, processctrl.LaunchContext{})
	fatal(err)
	log.Printf(`launched %s pid=%d`, *bundle, pid)
}

func runKill(args []string) {
	fs, udid := newFlagSet(`kill`)
	bundle := fs.String(`bundle`, ``, `bundle identifier (required)`)
	fs.Parse(args)
	if *bundle == `` {
		fmt.Fprintln(os.Stderr, "kill: -bundle is required")
		fs.Usage()
		os.Exit(1)
	}

	dev := openDevice(*udid)
	procs, err := dev.ListProcesses()
	fatal(err)
	killed := 0
	for _, p := range procs {
		if p.BundleIdentifier == *bundle {
			fatal(dev.Kill(p.Pid))
			log.Printf(`killed %s pid=%d`, *bundle, p.Pid)
			killed++
		}
	}
	if killed == 0 {
		fmt.Fprintf(os.Stderr, "%s is not running\n", *bundle)
		os.Exit(1)
	}
}

func runInstall(args []string) {
	fs, udid := newFlagSet(`install`)
	path := fs.String(`path`, ``, `IPA file path (required)`)
	fs.Parse(args)
	if *path == `` {
		fmt.Fprintln(os.Stderr, "install: -path is required")
		fs.Usage()
		os.Exit(1)
	}

	dev := openDevice(*udid)
	fatal(dev.Install(*path))
	log.Printf(`installed %s`, *path)
}

func runUninstall(args []string) {
	fs, udid := newFlagSet(`uninstall`)
	bundle := fs.String(`bundle`, ``, `bundle identifier (required)`)
	fs.Parse(args)
	if *bundle == `` {
		fmt.Fprintln(os.Stderr, "uninstall: -bundle is required")
		fs.Usage()
		os.Exit(1)
	}

	dev := openDevice(*udid)
	fatal(dev.Uninstall(*bundle))
	log.Printf(`uninstalled %s`, *bundle)
}

type pathList []string

func (p *pathList) String() string  { return strings.Join(*p, `,`) }
func (p *pathList) Set(v string) error { *p = append(*p, v); return nil }

func runPull(args []string) {
	fs, udid := newFlagSet(`pull`)
	bundle := fs.String(`bundle`, ``, `bundle identifier (required)`)
	remote := fs.String(`remote`, ``, `remote path on device (required)`)
	local  := fs.String(`local`, ``, `local destination path (required)`)
	fs.Parse(args)
	if *bundle == `` || *remote == `` || *local == `` {
		fmt.Fprintln(os.Stderr, "pull: -bundle, -remote and -local are required")
		fs.Usage()
		os.Exit(1)
	}

	dev := openDevice(*udid)
	has, err := dev.HouseArrestService()
	fatal(err)
	afcSvc, err := has.AfcService(*bundle)
	fatal(err)

	items, err := afcSvc.List(*remote, 0)
	fatal(err)
	if len(items) > 0 {
		for _, it := range items {
			fatal(pullFile(afcSvc, it.Name, filepath.Join(*local, filepath.Base(it.Name))))
		}
	} else {
		fatal(pullFile(afcSvc, *remote, *local))
	}
}

func runPush(args []string) {
	fs, udid := newFlagSet(`push`)
	bundle := fs.String(`bundle`, ``, `bundle identifier (required)`)
	local  := fs.String(`local`, ``, `local source path (required)`)
	remote := fs.String(`remote`, ``, `remote destination path on device (required)`)
	fs.Parse(args)
	if *bundle == `` || *local == `` || *remote == `` {
		fmt.Fprintln(os.Stderr, "push: -bundle, -local and -remote are required")
		fs.Usage()
		os.Exit(1)
	}

	info, err := os.Stat(*local)
	fatal(err)
	dev := openDevice(*udid)
	has, err := dev.HouseArrestService()
	fatal(err)
	afcSvc, err := has.AfcService(*bundle)
	fatal(err)
	if info.IsDir() {
		fatal(pushDir(afcSvc, *local, *remote))
	} else {
		fatal(pushFile(afcSvc, *local, *remote))
	}
}

func runRemove(args []string) {
	fs, udid := newFlagSet(`remove`)
	bundle := fs.String(`bundle`, ``, `bundle identifier (required)`)
	remote := fs.String(`remote`, ``, `remote path to remove (required)`)
	fs.Parse(args)
	if *bundle == `` || *remote == `` {
		fmt.Fprintln(os.Stderr, "remove: -bundle and -remote are required")
		fs.Usage()
		os.Exit(1)
	}

	dev := openDevice(*udid)
	has, err := dev.HouseArrestService()
	fatal(err)
	afcSvc, err := has.AfcService(*bundle)
	fatal(err)
	fatal(afcSvc.Remove(*remote))
	log.Printf(`removed %s`, *remote)
}

func runProcess(args []string) {
	fs, udid := newFlagSet(`process`)
	doList         := fs.Bool(`list`, false, `list running processes`)
	killPid        := fs.Int(`kill`, 0, `kill process by PID`)
	killBundle     := fs.String(`kill-bundle`, ``, `kill process by bundle identifier`)
	relaunchBundle := fs.String(`relaunch`, ``, `kill then relaunch app by bundle identifier`)
	fs.Parse(args)

	dev := openDevice(*udid)

	switch {
	case *doList:
		procs, err := dev.ListProcesses()
		fatal(err)
		for _, p := range procs {
			kind := "daemon"
			if p.IsApplication {
				kind = "app"
			}
			bundle := p.BundleIdentifier
			if bundle == `` {
				bundle = `-`
			}
			fmt.Printf("%-8d %-8s %-60s %s\n", p.Pid, kind, bundle, p.Name)
		}

	case *killPid != 0:
		fatal(dev.Kill(*killPid))
		log.Printf(`killed pid=%d`, *killPid)

	case *killBundle != ``:
		procs, err := dev.ListProcesses()
		fatal(err)
		killed := 0
		for _, p := range procs {
			if p.BundleIdentifier == *killBundle {
				fatal(dev.Kill(p.Pid))
				log.Printf(`killed %s pid=%d`, *killBundle, p.Pid)
				killed++
			}
		}
		if killed == 0 {
			fmt.Fprintf(os.Stderr, "%s is not running\n", *killBundle)
			os.Exit(1)
		}

	case *relaunchBundle != ``:
		procs, err := dev.ListProcesses()
		fatal(err)
		for _, p := range procs {
			if p.BundleIdentifier == *relaunchBundle {
				fatal(dev.Kill(p.Pid))
				log.Printf(`killed %s pid=%d`, *relaunchBundle, p.Pid)
			}
		}
		pid, err := dev.Launch(*relaunchBundle, processctrl.LaunchContext{})
		fatal(err)
		log.Printf(`launched %s pid=%d`, *relaunchBundle, pid)

	default:
		fmt.Fprintln(os.Stderr, "process: specify -list, -kill <pid>, -kill-bundle <bundle>, or -relaunch <bundle>")
		fs.Usage()
		os.Exit(1)
	}
}

func runList(args []string) {
	fs, udid := newFlagSet(`list`)
	raw := fs.Bool(`raw`, false, `output raw plist (XML)`)
	fs.Parse(args)

	dev := openDevice(*udid)

	apps, err := dev.ListApplications()
	fatal(err)

	if *raw {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent(``, `  `)
		fatal(enc.Encode(apps))
		return
	}

	for bid, app := range apps {
		fmt.Printf("%s\t%s\n", bid, app.CFBundleDisplayName)
	}
}

// ANSI color codes
const (
	ansiReset   = "\033[0m"
	ansiRed     = "\033[31m"
	ansiBoldRed = "\033[1;31m"
	ansiYellow  = "\033[33m"
	ansiCyan    = "\033[36m"
	ansiLightGray = "\033[37m" // Info    — light gray
	ansiDimGray   = "\033[90m" // Debug   — dim gray
	ansiDimGreen  = "\033[2;32m" // Notice — dim green (most common)
)

// logLevels defines Apple Unified Logging severity order (lowest to highest).
// "default" is the wire name for Notice in syslog relay output.
var logLevels = []string{`debug`, `info`, `notice`, `error`, `fault`}

// logLevelRank maps level name (lower-case) to its rank in logLevels.
var logLevelRank = func() map[string]int {
	m := make(map[string]int, len(logLevels))
	for i, l := range logLevels {
		m[l] = i
	}
	return m
}()

// extractLevel parses the level tag from a syslog line, e.g. "<Error>" -> "Error".
func extractLevel(line string) string {
	s := strings.Index(line, `<`)
	e := strings.Index(line, `>`)
	if s < 0 || e <= s {
		return ""
	}
	return line[s+1 : e]
}

// levelColor returns an ANSI prefix for a syslog level tag like "<Error>".
func levelColor(line string) string {
	switch extractLevel(line) {
	case `Fault`:
		return ansiBoldRed
	case `Error`:
		return ansiRed
	case `Notice`:
		return ansiDimGreen
	case `Info`:
		return ansiLightGray
	case `Debug`:
		return ansiDimGray
	default:
		return ""
	}
}

type logWriter struct {
	w        io.Writer
	color    bool
	re       *regexp.Regexp
	minLevel int // -1 = show all; otherwise minimum rank in logLevels
}

func (c *logWriter) Write(p []byte) (int, error) {
	// syslog.Streaming delivers one complete message per Write call.
	msg := strings.TrimRight(string(p), "\n")

	// level filter: skip messages below the minimum rank
	if c.minLevel >= 0 {
		lvl := strings.ToLower(extractLevel(msg))
		if rank, ok := logLevelRank[lvl]; ok && rank < c.minLevel {
			return len(p), nil
		}
	}

	// regex filter
	if c.re != nil && !c.re.MatchString(msg) {
		return len(p), nil
	}

	if c.color {
		if color := levelColor(msg); color != "" {
			fmt.Fprint(c.w, color+msg+ansiReset+"\n")
			return len(p), nil
		}
	}

	fmt.Fprintln(c.w, msg)
	return len(p), nil
}

func runLog(args []string) {
	fs, udid := newFlagSet(`log`)
	color := fs.Bool(`color`, false, `colorize output by log level`)
	match := fs.String(`match`, ``, `only show lines matching this regex`)
	level := fs.String(`level`, ``, `minimum log level: debug|info|notice|error|fault`)
	fs.Parse(args)

	var re *regexp.Regexp
	if *match != `` {
		var err error
		re, err = regexp.Compile(*match)
		if err != nil {
			fmt.Fprintf(os.Stderr, "log: invalid -match regex: %v\n", err)
			os.Exit(1)
		}
	}

	minLevel := -1
	if *level != `` {
		l := strings.ToLower(strings.TrimSpace(*level))
		rank, ok := logLevelRank[l]
		if !ok {
			fmt.Fprintf(os.Stderr, "log: unknown -level %q, valid: %s\n", l, strings.Join(logLevels, "|"))
			os.Exit(1)
		}
		minLevel = rank
	}

	dev := openDevice(*udid)
	fatal(dev.Logcat(&logWriter{w: os.Stdout, color: *color, re: re, minLevel: minLevel}))
}

// commaInt formats an integer with thousand separators (e.g. 1234567 → "1,234,567").
func commaInt(n int64) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		s = s[1:]
	}
	var buf []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, byte(c))
	}
	if n < 0 {
		return "-" + string(buf)
	}
	return string(buf)
}

// formatSize returns a human-readable byte count (e.g. "1.23 MB").
func formatSize(n int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case n >= GB:
		return fmt.Sprintf("%.2f GB", float64(n)/GB)
	case n >= MB:
		return fmt.Sprintf("%.2f MB", float64(n)/MB)
	case n >= KB:
		return fmt.Sprintf("%.2f KB", float64(n)/KB)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// openAfc returns an AFC service for the given bundle ID, or the system AFC
// (which exposes DCIM/, Downloads/, etc.) when bundle is empty.
// On HouseArrest failure it queries the app list to give an actionable error.
func openAfc(dev *device.Service, bundle string) (*afc.Service, error) {
	if bundle == "" {
		return dev.AfcService()
	}

	has, err := dev.HouseArrestService()
	if err != nil {
		return nil, err
	}
	svc, err := has.AfcService(bundle)
	if err == nil {
		return svc, nil
	}

	// Diagnose: check whether the bundle exists and has file-sharing enabled.
	apps, listErr := dev.ListApplications()
	if listErr != nil {
		return nil, err // return original error if list also fails
	}
	app, found := apps[bundle]
	if !found {
		return nil, fmt.Errorf("%w\n  app %q is not installed on this device", err, bundle)
	}
	if !app.UIFileSharingEnabled && !app.UISupportsDocumentBrowser {
		return nil, fmt.Errorf("%w\n  %q (%s) has not enabled file sharing (UIFileSharingEnabled / UISupportsDocumentBrowser not set)", err, app.CFBundleDisplayName, bundle)
	}
	return nil, err
}

// fsMatchExt reports whether name matches any of the comma-separated extensions
// (e.g. ".mp4,.mov"). An empty extFilter matches everything.
func fsMatchExt(name, extFilter string) bool {
	if extFilter == "" {
		return true
	}
	ext := strings.ToLower(path.Ext(name))
	for _, e := range strings.Split(extFilter, ",") {
		e = strings.TrimSpace(strings.ToLower(e))
		if e != "" && !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		if ext == e {
			return true
		}
	}
	return false
}

// parseSizeFlag parses strings like "10MB", "500KB", "1GB" into bytes.
// Returns 0 on empty input.
func parseSizeFlag(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	upper := strings.ToUpper(s)
	multiplier := int64(1)
	num := upper
	switch {
	case strings.HasSuffix(upper, "GB"):
		multiplier = 1024 * 1024 * 1024
		num = upper[:len(upper)-2]
	case strings.HasSuffix(upper, "MB"):
		multiplier = 1024 * 1024
		num = upper[:len(upper)-2]
	case strings.HasSuffix(upper, "KB"):
		multiplier = 1024
		num = upper[:len(upper)-2]
	case strings.HasSuffix(upper, "B"):
		num = upper[:len(upper)-1]
	}
	var v int64
	if _, err := fmt.Sscan(num, &v); err != nil {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return v * multiplier, nil
}

// --- fs ls ---

func runFsLs(args []string) {
	fs := flag.NewFlagSet("fs ls", flag.ExitOnError)
	udid       := fs.String("udid", "", "device UDID (default: auto-select)")
	bundle     := fs.String("bundle", "", "app bundle identifier (omit to access system AFC / DCIM)")
	dir        := fs.String("dir", "", "remote directory to list (default: / for bundle, DCIM/ for system)")
	extFilter  := fs.String("ext", "", "comma-separated extensions to show, e.g. mp4,mov")
	minSizeStr := fs.String("min-size", "", "only show files >= this size, e.g. 1MB")
	sortBy     := fs.String("sort", "", "sort order: size|name|time (default: no sort, stream as received)")
	depth      := fs.Int("depth", 0, "max directory depth to recurse (0 = unlimited)")
	fs.Parse(args)

	if *dir == "" {
		if *bundle == "" {
			*dir = "DCIM/"
		} else {
			*dir = "/"
		}
	}

	minSize, err := parseSizeFlag(*minSizeStr)
	fatal(err)

	dev := openDevice(*udid)
	afcSvc, err := openAfc(dev, *bundle)
	fatal(err)

	printEntry := func(f *afc.FileStat) {
		mtime := "-"
		if f.Mtime != nil {
			mtime = (*time.Time)(f.Mtime).Local().Format("2006-01-02 15:04:05")
		}
		name := f.Name
		if f.IsDir() {
			name += "/"
		}
		fmt.Printf("%14s  %-19s  %s\n", commaInt(f.Size), mtime, name)
	}

	// dirs always pass the filter; files must match ext and min-size
	match := func(it *afc.FileStat) bool {
		if it.IsDir() {
			return true
		}
		return fsMatchExt(it.Name, *extFilter) && it.Size >= minSize
	}

	fmt.Printf("%14s  %-19s  %s\n", "SIZE", "MODIFIED", "PATH")
	fmt.Println(strings.Repeat("-", 80))

	total := int64(0)
	count := 0

	if *sortBy == "" {
		// streaming: print each entry as it arrives, summary at end
		fatal(afcSvc.Walk(*dir, *depth, func(it *afc.FileStat) {
			if !match(it) {
				return
			}
			printEntry(it)
			if !it.IsDir() {
				total += it.Size
				count++
			}
		}))
	} else {
		// buffered: collect all entries, sort, then print
		var entries []*afc.FileStat
		fatal(afcSvc.Walk(*dir, *depth, func(it *afc.FileStat) {
			if match(it) {
				entries = append(entries, it)
			}
		}))
		switch *sortBy {
		case "name":
			sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
		case "time":
			sort.Slice(entries, func(i, j int) bool {
				if entries[i].Mtime == nil || entries[j].Mtime == nil {
					return false
				}
				return (*time.Time)(entries[i].Mtime).After(*(*time.Time)(entries[j].Mtime))
			})
		case "size":
			sort.Slice(entries, func(i, j int) bool { return entries[i].Size > entries[j].Size })
		}
		for _, f := range entries {
			printEntry(f)
			if !f.IsDir() {
				total += f.Size
				count++
			}
		}
	}

	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%d file(s)  total %s bytes\n", count, commaInt(total))
}

// --- fs du ---

func runFsDu(args []string) {
	fs := flag.NewFlagSet("fs du", flag.ExitOnError)
	udid      := fs.String("udid", "", "device UDID (default: auto-select)")
	bundle    := fs.String("bundle", "", "app bundle identifier (omit to access system AFC / DCIM)")
	dir       := fs.String("dir", "", "remote directory to analyse (default: / for bundle, DCIM/ for system)")
	recursive := fs.Bool("recursive", false, "recurse into subdirectories to compute exact sizes (slow for large libraries)")
	topN      := fs.Int("top", 0, "show top N entries by size (0 = all)")
	fs.Parse(args)

	if *dir == "" {
		if *bundle == "" {
			*dir = "DCIM/"
		} else {
			*dir = "/"
		}
	}

	dev := openDevice(*udid)
	afcSvc, err := openAfc(dev, *bundle)
	fatal(err)

	// Non-recursive: just list immediate children and show file counts / known sizes.
	// Recursive: walk everything and bucket by first-level subdir.
	type entry struct {
		name    string
		size    int64
		nfiles  int
		isDir   bool
	}

	var entries []entry

	if !*recursive {
		// shallow listing: list immediate children, then for each subdir
		// do a non-recursive list to count its files.
		items, err := afcSvc.List(*dir, 1)
		fatal(err)
		for _, it := range items {
			e := entry{name: it.Name, isDir: it.IsDir()}
			if it.IsDir() {
				children, err := afcSvc.List(it.Name, 1)
				if err == nil {
					for _, c := range children {
						if !c.IsDir() {
							e.nfiles++
							e.size += c.Size
						}
					}
				}
			} else {
				e.nfiles = 1
				e.size = it.Size
			}
			entries = append(entries, e)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].nfiles > entries[j].nfiles })
	} else {
		items, err := afcSvc.List(*dir, 0)
		fatal(err)
		buckets := map[string]*entry{}
		for _, it := range items {
			if it.IsDir() {
				continue
			}
			rel := strings.TrimPrefix(it.Name, strings.TrimRight(*dir, "/")+"/")
			parts := strings.SplitN(rel, "/", 2)
			key := path.Join(*dir, parts[0])
			if _, ok := buckets[key]; !ok {
				buckets[key] = &entry{name: key}
			}
			buckets[key].size += it.Size
			buckets[key].nfiles++
		}
		for _, e := range buckets {
			entries = append(entries, *e)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].size > entries[j].size })
	}

	if *topN > 0 && len(entries) > *topN {
		entries = entries[:*topN]
	}

	fmt.Println(strings.Repeat("-", 60))
	totalSize := int64(0)
	totalFiles := 0
	if *recursive {
		fmt.Printf("%-12s  %-8s  %s\n", "SIZE", "FILES", "PATH")
		fmt.Println(strings.Repeat("-", 60))
		for _, e := range entries {
			fmt.Printf("%-12s  %-8d  %s\n", formatSize(e.size), e.nfiles, e.name)
			totalSize += e.size
			totalFiles += e.nfiles
		}
	} else {
		fmt.Printf("%-8s  %-12s  %s\n", "FILES", "SIZE", "PATH")
		fmt.Println(strings.Repeat("-", 60))
		for _, e := range entries {
			suffix := ""
			if e.isDir {
				suffix = "/"
			}
			fmt.Printf("%-8d  %-12s  %s%s\n", e.nfiles, formatSize(e.size), e.name, suffix)
			totalSize += e.size
			totalFiles += e.nfiles
		}
	}
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("total: %d file(s)  %s\n", totalFiles, formatSize(totalSize))
}

// --- fs clean ---

func runFsClean(args []string) {
	fs := flag.NewFlagSet("fs clean", flag.ExitOnError)
	udid       := fs.String("udid", "", "device UDID (default: auto-select)")
	bundle     := fs.String("bundle", "", "app bundle identifier (omit to access system AFC / DCIM)")
	dir        := fs.String("dir", "", "remote directory to scan (default: / for bundle, DCIM/ for system)")
	extFilter  := fs.String("ext", "", "comma-separated extensions to delete, e.g. mp4,mov,avi (required)")
	minSizeStr := fs.String("min-size", "", "only delete files >= this size, e.g. 1MB")
	dryRun     := fs.Bool("dry-run", false, "preview deletions without actually removing files")
	fs.Parse(args)

	if *dir == "" {
		if *bundle == "" {
			*dir = "DCIM/"
		} else {
			*dir = "/"
		}
	}

	if *extFilter == "" {
		fmt.Fprintln(os.Stderr, "fs clean: -ext is required")
		fs.Usage()
		os.Exit(1)
	}

	minSize, err := parseSizeFlag(*minSizeStr)
	fatal(err)

	dev := openDevice(*udid)
	afcSvc, err := openAfc(dev, *bundle)
	fatal(err)

	items, err := afcSvc.List(*dir, 0)
	fatal(err)

	var targets []*afc.FileStat
	for _, it := range items {
		if it.IsDir() {
			continue
		}
		if !fsMatchExt(it.Name, *extFilter) {
			continue
		}
		if it.Size < minSize {
			continue
		}
		targets = append(targets, it)
	}

	// sort by size descending for clear preview
	sort.Slice(targets, func(i, j int) bool { return targets[i].Size > targets[j].Size })

	if len(targets) == 0 {
		fmt.Println("no files matched — nothing to clean")
		return
	}

	totalSize := int64(0)
	for _, f := range targets {
		totalSize += f.Size
	}

	mode := "DELETE"
	if *dryRun {
		mode = "DRY-RUN"
	}
	fmt.Printf("[%s] %d file(s)  %s\n", mode, len(targets), formatSize(totalSize))
	fmt.Printf("%-12s  %s\n", "SIZE", "PATH")
	fmt.Println(strings.Repeat("-", 70))
	for _, f := range targets {
		fmt.Printf("%-12s  %s\n", formatSize(f.Size), f.Name)
	}
	fmt.Println(strings.Repeat("-", 70))

	if *dryRun {
		fmt.Println("dry-run: no files removed. re-run without -dry-run to delete.")
		return
	}

	fmt.Printf("\ntype YES to confirm deletion of %d file(s) (%s): ", len(targets), formatSize(totalSize))
	var answer string
	fmt.Fscan(os.Stdin, &answer)
	if answer != "YES" {
		fmt.Println("aborted")
		return
	}

	deleted := 0
	freed := int64(0)
	for _, f := range targets {
		if err := afcSvc.Remove(f.Name); err != nil {
			log.Printf("remove %s: %v", f.Name, err)
		} else {
			freed += f.Size
			deleted++
		}
	}
	fmt.Printf("deleted %d file(s), freed %s\n", deleted, formatSize(freed))
}

// --- fs pull ---

func runFsPull(args []string) {
	fs := flag.NewFlagSet("fs pull", flag.ExitOnError)
	udid       := fs.String("udid", "", "device UDID (default: auto-select)")
	bundle     := fs.String("bundle", "", "app bundle identifier (omit to access system AFC / DCIM)")
	dir        := fs.String("dir", "", "remote directory to download (default: DCIM/ for system, / for bundle)")
	local      := fs.String("local", ".", "local destination directory")
	extFilter  := fs.String("ext", "", "only download files with these extensions, e.g. jpg,heic,mp4")
	minSizeStr := fs.String("min-size", "", "only download files >= this size, e.g. 1MB")
	flat       := fs.Bool("flat", false, "save all files into local dir directly (no subdirectories)")
	fs.Parse(args)

	if *dir == "" {
		if *bundle == "" {
			*dir = "DCIM/"
		} else {
			*dir = "/"
		}
	}

	minSize, err := parseSizeFlag(*minSizeStr)
	fatal(err)

	dev := openDevice(*udid)
	afcSvc, err := openAfc(dev, *bundle)
	fatal(err)

	items, err := afcSvc.List(*dir, 0)
	fatal(err)

	var targets []*afc.FileStat
	for _, it := range items {
		if it.IsDir() {
			continue
		}
		if !fsMatchExt(it.Name, *extFilter) {
			continue
		}
		if it.Size < minSize {
			continue
		}
		targets = append(targets, it)
	}

	if len(targets) == 0 {
		fmt.Println("no files matched")
		return
	}

	totalSize := int64(0)
	for _, f := range targets {
		totalSize += f.Size
	}
	fmt.Printf("downloading %d file(s)  %s\n", len(targets), formatSize(totalSize))

	downloaded, skipped := 0, 0
	for _, f := range targets {
		var localPath string
		if *flat {
			localPath = filepath.Join(*local, filepath.Base(f.Name))
		} else {
			rel := strings.TrimPrefix(f.Name, *dir)
			rel = strings.TrimPrefix(rel, "/")
			localPath = filepath.Join(*local, filepath.FromSlash(rel))
		}

		// skip if already exists with same size
		if info, err := os.Stat(localPath); err == nil && info.Size() == f.Size {
			log.Printf("skip %s (already exists)", localPath)
			skipped++
			continue
		}

		if err := pullFile(afcSvc, f.Name, localPath); err != nil {
			log.Printf("error: %v", err)
		} else {
			downloaded++
		}
	}
	fmt.Printf("done: %d downloaded, %d skipped\n", downloaded, skipped)
}

// --- fs dispatcher ---

func runFs(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, `Usage: devicetool fs <subcommand> [flags]

Subcommands:
  ls      list files, sorted by size
  du      show disk usage by directory
  pull    download files to local disk
  clean   delete files matching extension / size / age rules

Omit -bundle to access the system AFC (DCIM/, Downloads/, etc.).

Examples:
  # browse system photo library
  devicetool fs du
  devicetool fs ls -ext jpg,heic,mp4

  # download all photos/videos to ~/Downloads
  devicetool fs pull -local ~/Downloads -ext jpg,heic,mp4,mov

  # download only large videos
  devicetool fs pull -local ~/Downloads -ext mp4,mov -min-size 50MB

  # app sandbox
  devicetool fs ls -bundle com.example.app
  devicetool fs clean -bundle com.example.app -ext mp4,mov -dry-run
`)
		os.Exit(1)
	}

	sub, subArgs := args[0], args[1:]

	switch sub {
	case "ls":
		runFsLs(subArgs)
	case "du":
		runFsDu(subArgs)
	case "pull":
		runFsPull(subArgs)
	case "clean":
		runFsClean(subArgs)
	case "-h", "-help", "--help":
		runFs(nil) // prints usage and exits
	default:
		fmt.Fprintf(os.Stderr, "fs: unknown subcommand %q\n", sub)
		os.Exit(1)
	}
}

func runUnzip(args []string) {
	fs := flag.NewFlagSet(`unzip`, flag.ExitOnError)
	src  := fs.String(`src`, ``, `source zip file (required)`)
	dest := fs.String(`dest`, ``, `destination directory (required)`)
	fs.Parse(args)
	if *src == `` || *dest == `` {
		fmt.Fprintln(os.Stderr, "unzip: -src and -dest are required")
		fs.Usage()
		os.Exit(1)
	}
	fatal(unzipFile(*src, *dest))
	log.Printf(`unzipped %s -> %s`, *src, *dest)
}

// --- main ---

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	cmd, args := os.Args[1], os.Args[2:]

	switch cmd {
	case `snap`:
		runSnap(args)
	case `launch`:
		runLaunch(args)
	case `process`:
		runProcess(args)
	case `kill`:
		runKill(args)
	case `install`:
		runInstall(args)
	case `uninstall`:
		runUninstall(args)
	case `pull`:
		runPull(args)
	case `push`:
		runPush(args)
	case `remove`:
		runRemove(args)
	case `list`:
		runList(args)
	case `log`:
		runLog(args)
	case `unzip`:
		runUnzip(args)
	case `fs`:
		runFs(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", cmd)
		usage()
	}
}
