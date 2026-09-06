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
	"path/filepath"
	"regexp"
	"strings"
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

	items, err := afcSvc.List(*remote, true)
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
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", cmd)
		usage()
	}
}
