package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"github.com/larryhou/ix/api/afc"
	"github.com/larryhou/ix/api/device"
	"github.com/larryhou/ix/api/dvt/processctrl"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	cmdSnap      = `snap`
	cmdPull      = `pull`
	cmdPush      = `push`
	cmdLaunch    = `launch`
	cmdKill      = `kill`
	cmdInstall   = `install`
	cmdUninstall = `uninstall`
	cmdUnzip     = `unzip`
	cmdRemove    = `remove`
	cmdList      = `list`
	cmdLog       = `log`
)

type arrValue []string

func (x *arrValue) Set(v string) error {
	*x = append(*x, v)
	return nil
}

func (x *arrValue) String() string {
	return strings.Join(*x, `,`)
}

func init() {
	log.SetFlags(log.LstdFlags)
}

// requirePaths checks that at least n -path arguments were provided.
func requirePaths(paths []string, n int, cmd string) {
	if len(paths) < n {
		fmt.Fprintf(os.Stderr, "command %q requires %d -path argument(s), got %d\n", cmd, n, len(paths))
		os.Exit(1)
	}
}

// requireBundle checks that -bundle was provided.
func requireBundle(bundle, cmd string) {
	if bundle == `` {
		fmt.Fprintf(os.Stderr, "command %q requires -bundle\n", cmd)
		os.Exit(1)
	}
}

// --- file helpers ---

func getAllFiles(dirPth string, dirName string) ([]string, error) {
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
		outFile, err := os.OpenFile(fPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return err
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
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
		dir := filepath.Dir(rel)
		if dir != "." {
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

// --- commands ---

func cmdSnap_(dev *device.Service, outPath string) error {
	data, err := dev.ScreenShot()
	if err != nil {
		return err
	}
	if err = os.WriteFile(outPath, data, 0644); err != nil {
		return err
	}
	log.Printf(`screenshot saved to %s (%d bytes)`, outPath, len(data))
	return nil
}

func cmdLaunch_(dev *device.Service, bundleid string) error {
	pid, err := dev.Launch(bundleid, processctrl.LaunchContext{})
	if err != nil {
		return err
	}
	log.Printf(`launched %s pid=%d`, bundleid, pid)
	return nil
}

func cmdKill_(dev *device.Service, bundleid string) error {
	procs, err := dev.ListProcesses()
	if err != nil {
		return err
	}
	killed := 0
	for _, p := range procs {
		if p.BundleIdentifier == bundleid {
			if err = dev.Kill(p.Pid); err != nil {
				return fmt.Errorf("kill pid %d: %w", p.Pid, err)
			}
			log.Printf(`killed %s pid=%d`, bundleid, p.Pid)
			killed++
		}
	}
	if killed == 0 {
		return fmt.Errorf("%s is not running", bundleid)
	}
	return nil
}

func cmdInstall_(dev *device.Service, path string) error {
	if err := dev.Install(path); err != nil {
		return err
	}
	log.Printf(`installed %s`, path)
	return nil
}

func cmdUninstall_(dev *device.Service, bundleid string) error {
	if err := dev.Uninstall(bundleid); err != nil {
		return err
	}
	log.Printf(`uninstalled %s`, bundleid)
	return nil
}

func cmdPull_(dev *device.Service, bundleID, remotePath, localPath string) error {
	has, err := dev.HouseArrestService()
	if err != nil {
		return err
	}
	afcSvc, err := has.AfcService(bundleID)
	if err != nil {
		return err
	}

	items, err := afcSvc.List(remotePath, true)
	if err != nil {
		return err
	}

	if len(items) > 0 {
		// remotePath is a directory — pull all items into localPath/
		for _, it := range items {
			local := filepath.Join(localPath, filepath.Base(it.Name))
			if err = pullFile(afcSvc, it.Name, local); err != nil {
				return err
			}
		}
	} else {
		// remotePath is a single file
		if err = pullFile(afcSvc, remotePath, localPath); err != nil {
			return err
		}
	}
	return nil
}

func cmdPush_(dev *device.Service, bundleID, localPath, remotePath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("local path %s: %w", localPath, err)
	}
	has, err := dev.HouseArrestService()
	if err != nil {
		return err
	}
	afcSvc, err := has.AfcService(bundleID)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return pushDir(afcSvc, localPath, remotePath)
	}
	return pushFile(afcSvc, localPath, remotePath)
}

func cmdRemove_(dev *device.Service, bundleID, remotePath string) error {
	has, err := dev.HouseArrestService()
	if err != nil {
		return err
	}
	afcSvc, err := has.AfcService(bundleID)
	if err != nil {
		return err
	}
	if err = afcSvc.Remove(remotePath); err != nil {
		return err
	}
	log.Printf(`removed %s`, remotePath)
	return nil
}

func cmdLog_(dev *device.Service) error {
	return dev.Logcat(os.Stdout)
}

func cmdList_(dev *device.Service) error {
	apps, err := dev.ListApplications()
	if err != nil {
		return err
	}
	for bid, app := range apps {
		fmt.Printf("%s\t%s\n", bid, app.CFBundleDisplayName)
	}
	return nil
}

// --- main ---

func main() {
	var (
		udid    string
		command string
		bundle  string
		paths   arrValue
	)

	flag.StringVar(&udid, `udid`, ``, `device UDID (default: auto-select)`)
	flag.StringVar(&command, `command`, ``, `command: snap | launch | kill | install | uninstall | pull | push | remove | unzip | list | log`)
	flag.StringVar(&bundle, `bundle`, ``, `application bundle id`)
	flag.Var(&paths, `path`, `file/directory path (repeatable)")`)
	flag.Parse()

	if command == cmdUnzip {
		requirePaths(paths, 2, cmdUnzip)
		if err := unzipFile(paths[0], paths[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		log.Printf(`unzipped %s -> %s`, paths[0], paths[1])
		return
	}

	dev, err := device.New(udid)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var cmdErr error
	switch command {
	case cmdSnap:
		outPath := `/tmp/screenshot.png`
		if len(paths) > 0 {
			outPath = paths[0]
		}
		cmdErr = cmdSnap_(dev, outPath)
	case cmdLaunch:
		requireBundle(bundle, command)
		cmdErr = cmdLaunch_(dev, bundle)
	case cmdKill:
		requireBundle(bundle, command)
		cmdErr = cmdKill_(dev, bundle)
	case cmdInstall:
		requirePaths(paths, 1, command)
		cmdErr = cmdInstall_(dev, paths[0])
	case cmdUninstall:
		requireBundle(bundle, command)
		cmdErr = cmdUninstall_(dev, bundle)
	case cmdPull:
		requireBundle(bundle, command)
		requirePaths(paths, 2, command)
		cmdErr = cmdPull_(dev, bundle, paths[0], paths[1])
	case cmdPush:
		requireBundle(bundle, command)
		requirePaths(paths, 2, command)
		cmdErr = cmdPush_(dev, bundle, paths[0], paths[1])
	case cmdRemove:
		requireBundle(bundle, command)
		requirePaths(paths, 1, command)
		cmdErr = cmdRemove_(dev, bundle, paths[0])
	case cmdLog:
		cmdErr = cmdLog_(dev)
	case cmdList:
		cmdErr = cmdList_(dev)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", command)
		flag.Usage()
		os.Exit(1)
	}

	if cmdErr != nil {
		fmt.Fprintln(os.Stderr, cmdErr)
		os.Exit(1)
	}
}
