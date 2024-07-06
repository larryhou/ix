package deviceinfo

import (
	"bufio"
	"bytes"
	"github.com/larryhou/j3idevice/api/dvt/remotesvr"
	"github.com/larryhou/j3idevice/api/dvt/systemtap"
	"github.com/larryhou/j3idevice/api/util"
	"github.com/mitchellh/mapstructure"
	"howett.net/plist"
	"regexp"
	"strconv"
	"strings"
)

func New(svr *remotesvr.Service) (*Service, error) {
	id, err := svr.OpenChannel(`com.apple.instruments.server.services.deviceinfo`)
	if err != nil {
		return nil, err
	}

	s := &Service{
		ch: svr.GetChannel(id),
		sv: svr,
	}

	return s, nil
}

type Service struct {
	ch *remotesvr.DTXChannel
	sv *remotesvr.Service
}

func (x *Service) ReadDir(name string) ([]string, error) {
	err := x.ch.Send(`directoryListingForPath:`, new(remotesvr.ArgumentAux).Obj(name), true)
	if err != nil {return nil, err}
	var out []string
	rsp, err := util.Cast[[]any](x.ch.Recv(nil))
	if err == nil {
		for _, name := range rsp {
			out = append(out, name.(string))
		}
		return out, nil
	}

	return nil, err
}

func (x *Service) GetProcName(pid int) (string, error) {
	err := x.ch.Send(`execnameForPid:`, new(remotesvr.ArgumentAux).Obj(pid), true)
	if err == nil {
		return util.Cast[string](x.ch.Recv(nil))
	}

	return ``, err
}

func (x *Service) ListProcesses() ([]*Process, error) {
	err := x.ch.Send(`runningProcesses`, new(remotesvr.ArgumentAux), true)
	if err != nil {return nil, err}
	rsp, err := x.ch.Recv(nil)
	if err == nil {
		var out []*Process
		err = mapstructure.Decode(rsp, &out)
		if err == nil {
			return out, nil
		}
	}

	return nil, err
}

func (x *Service) SystemInfomation() (any, error) {
	return x.Get(`systemInformation`)
}

func (x *Service) HardwareInformation() (any, error) {
	return util.Cast[map[string]any](x.Get(`hardwareInformation`))
}

func (x *Service) NetworkInformation() (any, error) {
	return util.Cast[map[string]any](x.Get(`networkInformation`))
}

func (x *Service) MachTimeInfo() (any, error) {
	return x.Get(`machTimeInfo`)
}

func (x *Service) MachKernelName() (any, error) {
	return util.Cast[map[string]any](x.Get(`machKernelName`))
}

func (x *Service) KpepDatabase() (any, error) {
	rsp, err := x.Get(`kpepDatabase`)
	if raw, ok := rsp.([]byte); ok {
		var out map[string]any
		return out, plist.NewDecoder(bytes.NewReader(raw)).Decode(&out)
	}

	return nil, err
}

func (x *Service) TraceCodesFile() (map[int]string, error) {
	rsp, err := x.Get(`traceCodesFile`)
	if data, ok := rsp.(string); ok {
		out := make(map[int]string)
		p := regexp.MustCompile(`\s+`)
		for s := bufio.NewScanner(bytes.NewBufferString(data)); s.Scan(); {
			kv := p.Split(s.Text(), 2)
			id, _ := strconv.ParseInt(strings.TrimSpace(kv[0]), 0, 32)
			out[int(id)] = strings.TrimSpace(kv[1])
		}
		return out, nil
	}

	return nil, err
}

func (x *Service) SysmonProcessAttributes() (map[any]struct{}, error) {
	return util.Cast[map[any]struct{}](x.Get(`sysmonProcessAttributes`))
}

func (x *Service) SysmonSystemAttributes() (map[any]struct{}, error) {
	return util.Cast[map[any]struct{}](x.Get(`sysmonSystemAttributes`))
}

func (x *Service) Get(name string) (any, error) {
	err := x.ch.Send(name, new(remotesvr.ArgumentAux), true)
	if err != nil {return nil, err}
	return x.ch.Recv(nil)
}

func (x *Service) GetUidName(uid int) (string, error) {
	err := x.ch.Send(`nameForUID:`, new(remotesvr.ArgumentAux).Obj(uid), true)
	if err != nil {return "", err}
	return util.Cast[string](x.ch.Recv(nil))
}

func (x *Service) GetGidName(gid int) (string, error) {
	err := x.ch.Send(`nameForGID:`, new(remotesvr.ArgumentAux).Obj(gid), true)
	if err != nil {return "", err}
	return util.Cast[string](x.ch.Recv(nil))
}

func (x *Service) SystemTap() (*systemtap.Service, error) {
	sys, err := x.SysmonSystemAttributes()
	if err != nil {return nil, err}

	proc, err := x.SysmonProcessAttributes()
	if err != nil {return nil, err}

	keys := func(v map[any]struct{}) []any {
		var r []any
		for k := range v {
			r = append(r, k)
		}
		return r
	}
	return systemtap.New(x.sv, map[string]any{
		`ur`:             500,
		`bm`:             0,
		`procAttrs`:      keys(proc),
		`sysAttrs`:       keys(sys),
		`cpuUsage`:       true,
		`sampleInterval`: 500000000,
	})
}