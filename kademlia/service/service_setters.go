package service

import "context"

func (s *Service) SetOnSeen(h SeenHook)              { s.hmu.Lock(); s.OnSeen = h; s.hmu.Unlock() }
func (s *Service) SetOnFindNode(h FindNodeHandler)   { s.hmu.Lock(); s.OnFindNode = h; s.hmu.Unlock() }
func (s *Service) SetOnStore(h StoreHandler)         { s.hmu.Lock(); s.OnStore = h; s.hmu.Unlock() }
func (s *Service) SetOnFindValue(h FindValueHandler) { s.hmu.Lock(); s.OnFindValue = h; s.hmu.Unlock() }
func (s *Service) SetOnDumpRT(h DumpRTHandler)       { s.hmu.Lock(); s.OnDumpRT = h; s.hmu.Unlock() }

func (s *Service) SetOnAdminPut(h func([]byte) ([20]byte, error)) {
	s.hmu.Lock()
	s.OnAdminPut = h
	s.hmu.Unlock()
}
func (s *Service) SetOnAdminGet(h func(context.Context, [20]byte) ([]byte, bool)) {
	s.hmu.Lock()
	s.OnAdminGet = h
	s.hmu.Unlock()
}
func (s *Service) SetOnAdminForget(h func([20]byte) bool) {
	s.hmu.Lock()
	s.OnAdminForget = h
	s.hmu.Unlock()
}
func (s *Service) SetOnExit(h func())            { s.hmu.Lock(); s.OnExit = h; s.hmu.Unlock() }
func (s *Service) SetOnRefresh(h func([20]byte)) { s.hmu.Lock(); s.OnRefresh = h; s.hmu.Unlock() }
