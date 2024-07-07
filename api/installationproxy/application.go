package installationproxy

import (
	"archive/zip"
	"errors"
	"github.com/larryhou/j3idevice/api/afc"
	"github.com/larryhou/j3idevice/api/j3"
	"github.com/larryhou/j3idevice/api/j3/plist"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	ServiceName = `com.apple.mobile.installation_proxy`
)

const (
	tmpIPAFilename = `/` + j3.ProgramName + `.ipa`
)

func New(service *plist.Service) *Service {
	return &Service{Service: service}
}

type Service struct {
	*plist.Service
}

func (x *Service) List() (*ListResponse, error) {
	req := &ListRequest{
		Command: `Lookup`,
		ClientOptions: &ClientOptions{
			ApplicationType: `Any`,
		},
	}

	rsp := &ListResponse{}
	return rsp, x.Get(req, rsp)
}

func (x *Service) Install(ipaname string, afcSvc *afc.Service) error {
	i, err := os.Stat(ipaname)
	if err != nil {return err}
	size := i.Size()
	if i.IsDir() {
		if !strings.HasSuffix(i.Name(), `.app`) {
			return errors.New(`BAD APP`)
		}
		root := `Payload/` + filepath.Base(ipaname)
		pending := []string{ipaname}
		f, err := os.OpenFile(ipaname+`.zip`, os.O_CREATE | os.O_TRUNC | os.O_WRONLY, 0644)
		if err != nil {return err}
		defer f.Close()

		z := zip.NewWriter(f)
		defer z.Close()

		for len(pending) > 0 {
			dir := pending[0]
			pending = pending[1:]
			entries, err := os.ReadDir(dir)
			if err != nil {return err}
			for _, ent := range entries {
				location := filepath.Join(dir, ent.Name())
				name, _ := filepath.Rel(ipaname, location)
				name = path.Join(root, name)

				if ent.IsDir() { name += `/` }

				w, err := z.Create(name)
				if err != nil {return err}

				if ent.IsDir() {
					pending = append(pending, location)
					continue
				}

				r, err := os.Open(location)
				if err != nil {return err}
				_, err = io.Copy(w, r)
				r.Close()

				if err != nil {return err}
			}
		}

		ipaname = f.Name()
		inf, _ := f.Stat()
		size = inf.Size()
	}

	{
		defer func() {
			if i.IsDir() { os.Remove(ipaname) }
		}()

		h, err := afcSvc.Open(tmpIPAFilename, `w`)
		if err != nil {return err}
		defer h.Close()

		w, err := h.FileWriter(size)
		if err != nil {return err}

		r, err := os.Open(ipaname)
		if err != nil {return err}
		defer r.Close()

		_, err = io.Copy(w, r)
		if err != nil {return err}
	}

	defer afcSvc.Remove(tmpIPAFilename)

	err = x.Send(map[string]any{
		`Command`:     `Install`,
		`PackagePath`: tmpIPAFilename,
	})

	if err != nil {return err}
	return x.waitComplete(`INSTALL`)
}

func (x *Service) Uninstall(identifier string) error {
	req := &Request{
		Command: `Uninstall`,
		ClientOptions: &ClientOptions{
			ApplicationIdentifier: identifier,
		},
	}

	err := x.Send(req)
	if err != nil {return err}
	return x.waitComplete(identifier)
}

func (x *Service) waitComplete(label string) error {
	const success = `Complete`
	rsp := &ProgressResponse{}
	for rsp.Status != success {
		if err := x.Recv(rsp); err != nil {
			return err
		}
		if rsp.Status == success {
			rsp.PercentComplete = 100
		}

		log.Printf(`PROGRESS %s[%s]: %d%%`, label, rsp.Status, rsp.PercentComplete)
	}
	return nil
}