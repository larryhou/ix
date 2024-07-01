package remote

import "github.com/larryhou/j3idevice/api/lockdown"

func New(lds *lockdown.Service) (*Server, error) {
	s := &Server{
		lockdown: lds,
	}

	return s, s.connect()
}

type Server struct {
	lockdown *lockdown.Service
}

func (x *Server) connect() error {
	panic(``)
}
