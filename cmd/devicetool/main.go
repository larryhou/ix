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
	cmdSnap            = `snap`
	cmdPull            = `pull`
	cmdPush            = `push`
	cmdLaunch          = `launch`
	cmdLaunchAndReturn = `launchAndReturn`
	cmdKill            = `kill`
	cmdInstall         = `install`
	cmdUninstall       = `uninstall`
	cmdUnzip           = `unzip`
	cmdRemove          = `remove`
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

func Test(err error, msg ...string) {
	if err != nil {
		if len(msg) == 0 {
			panic(err)
		} else {
			panic(fmt.Sprintf(`%s %+v`, strings.Join(msg, ` -- `), err))
		}
	}
}

func GetAllFiles(dirPth string, dirName string) (files []string, err error) {
	fis, err := os.ReadDir(filepath.Clean(filepath.ToSlash(dirPth)))
	if err != nil {
		return nil, err
	}

	for _, f := range fis {
		_path := filepath.Join(dirName, f.Name())

		if f.IsDir() {
			fullPath := filepath.Join(dirPth, f.Name())
			fs, _ := GetAllFiles(fullPath, _path)
			files = append(files, fs...)
			continue
		} else {
			files = append(files, _path)
		}

	}

	return files, nil
}

func UnzipFile(src string, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	os.MkdirAll(dest, 0777)
	for _, file := range r.File {
		fPath := filepath.Join(dest, file.Name)
		if file.FileInfo().IsDir() {
			os.MkdirAll(fPath, file.Mode())
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fPath), 0755); err != nil {
			return err
		}
		outFile, err := os.OpenFile(fPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
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

func PullFile(afcSvc *afc.Service, bundleID string, localFile string, remoteFile string) error {
	println("remotePath:", remoteFile, localFile)
	h, err := afcSvc.Open(remoteFile, `r`)
	if err != nil {
		panic(err)
	}
	r, err := h.FileReader()
	if err != nil {
		panic(err)
	}
	f, err := os.OpenFile(localFile, os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		panic(err)
	}
	io.Copy(f, r)
	f.Close()
	h.Close()
	return nil
}

func PushFile(afcSvc *afc.Service, bundleID string, localFile string, remotePath string) error {
	h, err := afcSvc.Open(remotePath, `w`)
	if err != nil {
		panic(err)
	}
	i, _ := os.Stat(localFile)

	w, err := h.FileWriter(i.Size())
	if err != nil {
		panic(err)
	}

	f, err := os.Open(localFile)
	if err != nil {
		panic(err)
	}

	io.Copy(w, f)
	f.Close()
	h.Close()
	return nil
}

func PushDir(afcSvc *afc.Service, bundleID string, localPath string, remotePath string) error {

	allfiles, err := GetAllFiles(localPath, "")
	afcSvc.MkDir(remotePath)
	if err != nil {
		panic(err)
	}
	print("PushDir len ", len(allfiles))
	for i := 0; i < len(allfiles); i++ {
		fmt.Printf("Index: %d, Fruit: %s\n", i, allfiles[i])
		//locapath, remotepath
		localFilePath := filepath.Join(localPath, allfiles[i])
		remoteFilePath := filepath.Join(remotePath, allfiles[i])
		tmpdir := filepath.Dir(allfiles[i])
		if tmpdir != "." {
			//create new dir
			remoteFileDir := filepath.Join(remotePath, tmpdir)
			afcSvc.MkDir(remoteFileDir)
			println("mkdir rmotedir:", remoteFileDir)
		}
		//println("localFilePath, remoteFilePath", localFilePath, remoteFilePath)
		PushFile(afcSvc, bundleID, localFilePath, remoteFilePath)
	}
	return err
}

func pull(dev *device.Service, bundleID string, remotePath string, localPath string) error {
	println("remotePath:", remotePath, localPath)

	has, err := dev.HouseArrestService()
	if err != nil {
		panic(err)
	}
	afcSvc, err := has.AfcService(bundleID)

	out, err := afcSvc.List(remotePath, true)
	if err != nil {
		panic(err)
	}
	println("out:", len(out))
	if len(out) > 0 {
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			// 如果目录不存在，创建目录
			err = os.MkdirAll(localPath, 0777)
			if err != nil {
				fmt.Println("创建目录失败:", err)
			}
			fmt.Println("目录创建成功:", localPath)
		} else {
			fmt.Println("目录已存在:", localPath)
		}
		for _, it := range out {
			//log.Printf(`%s #%d`, it.Name, it.Size)
			fileName := filepath.Base(it.Name)
			localfileName := filepath.Join(localPath, fileName)
			PullFile(afcSvc, bundleID, localfileName, it.Name)
		}
	} else {
		dir := filepath.Dir(localPath)
		print("--local--dir-", dir)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			// 如果目录不存在，创建目录
			err = os.MkdirAll(dir, 0777)
			if err != nil {
				fmt.Println("创建目录失败:", err)
			}
			fmt.Println("目录创建成功:", localPath)
		} else {
			fmt.Println("目录已存在:", localPath)
		}
		PullFile(afcSvc, bundleID, localPath, remotePath)
	}

	return err
}
func push(dev *device.Service, bundleID string, localPath string, remotePath string) error {
	s, err := os.Stat(localPath)
	if err != nil {
		print("path is not exist:", localPath)
		return err
	}
	has, err := dev.HouseArrestService()
	if err != nil {
		panic(err)
	}
	afcSvc, err := has.AfcService(bundleID)
	if s.IsDir() {
		println("path is dir:", localPath)
		PushDir(afcSvc, bundleID, localPath, remotePath)
	} else {
		PushFile(afcSvc, bundleID, localPath, remotePath)
	}
	return nil
}
func install(dev *device.Service, path string) error {
	dev.Install(path)
	println("install success")
	return nil
}

func uninstall(dev *device.Service, bundleid string) error {
	dev.Uninstall(bundleid)
	println("Uninstall ipa done!")
	return nil
}

func launch(dev *device.Service, bundleid string) error {
	pid, err := dev.Launch(bundleid, processctrl.LaunchContext{})
	if err != nil {
		return err
	}
	log.Printf(`launched %s pid=%d`, bundleid, pid)
	return nil
}

func launchAndReturn(dev *device.Service, bundleid string) error {
	pid, err := dev.Launch(bundleid, processctrl.LaunchContext{})
	if err != nil {
		return err
	}
	log.Printf(`launched %s pid=%d`, bundleid, pid)
	return nil
}

func kill(dev *device.Service, bundleid string) error {
	pid, err := dev.Launch(bundleid, processctrl.LaunchContext{})
	println("kill pid", pid)
	dev.Kill(pid)
	println(err)
	return nil
}

func unzip(localPath string, pufferPath string) error {
	// 检查文件是否存在
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		fmt.Println("file not exist:", localPath)
	}
	// 检查是否是.zip文件
	if filepath.Ext(localPath) != ".zip" {
		fmt.Println("is not a zip file:", localPath)
	}
	// 解压.zip文件
	err := UnzipFile(localPath, pufferPath)
	if err != nil {
		fmt.Println("unzip fail:", err)
	}
	fmt.Println("unzip success")
	return nil
}
func remove(dev *device.Service, bundleID, aremotePath string) error {
	has, err := dev.HouseArrestService()
	if err != nil {
		panic(err)
	}
	afcSvc, err := has.AfcService(bundleID)
	if err != nil {
		panic(err)
	}
	afcSvc.Remove(aremotePath)
	return nil
}

//unistall install launch push

func main() {
	opts := struct {
		udid     string
		command  string
		bundle   string
		activity string
		typo     string
		path     []string
		point    []string
		swipe    int
		value    string
		code     string
		sn       string
	}{}

	flag.StringVar(&opts.udid, `udid`, ``, `device UDID (default: auto-select)`)
	flag.StringVar(&opts.command, `command`, ``, `command: snap | pull | push | launch | launchAndReturn | kill | install | uninstall | unzip | remove | list`)
	flag.StringVar(&opts.bundle, `bundle`, ``, `application bundle id`)
	flag.Var((*arrValue)(&opts.path), `path`, `file/directory path[s]`)
	flag.Parse()

	dev, err := device.New(opts.udid)
	if err != nil {
		panic(err)
	}

	switch opts.command {
	case cmdSnap:
		outPath := `/tmp/screenshot.png`
		if len(opts.path) > 0 {
			outPath = opts.path[0]
		}
		data, err := dev.ScreenShot()
		if err != nil {
			panic(err)
		}
		if err = os.WriteFile(outPath, data, 0644); err != nil {
			panic(err)
		}
		log.Printf(`screenshot saved to %s (%d bytes)`, outPath, len(data))
	case cmdLaunch:
		Test(launch(dev, opts.bundle))
	case cmdLaunchAndReturn:
		Test(launchAndReturn(dev, opts.bundle))
	case cmdInstall:
		Test(install(dev, opts.path[0]))
	case cmdUninstall:
		Test(uninstall(dev, opts.bundle))
	case cmdKill:
		Test(kill(dev, opts.bundle))
	case cmdUnzip:
		Test(unzip(opts.path[0], opts.path[1]))
	case cmdRemove:
		Test(remove(dev, opts.bundle, opts.path[0]))
	case cmdPull:
		Test(pull(dev, opts.bundle, opts.path[0], opts.path[1]))
	case cmdPush:
		Test(push(dev, opts.bundle, opts.path[0], opts.path[1]))
	}

	return

}
