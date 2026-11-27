package crc

import "time"

// SetAntiStall configures the stall timeout and monitor interval for the Anti-Stall resilience engine.
func (s *Server) SetAntiStall(timeout, interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if timeout > 0 {
		s.antiStallTimeout = timeout
	}
	if interval > 0 {
		s.antiStallInterval = interval
	}
}

// EnableAntiStall enables or disables the background anti-stall watchdog.
func (s *Server) EnableAntiStall(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.antiStallEnabled = enabled
}

func (s *Server) antiStallMonitorLoop() {
	defer s.wg.Done()

	s.mu.Lock()
	interval := s.antiStallInterval
	s.mu.Unlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.checkStalledFederates()
		}
	}
}

func (s *Server) checkStalledFederates() {
	s.mu.Lock()
	enabled := s.antiStallEnabled
	timeout := s.antiStallTimeout
	s.mu.Unlock()

	if !enabled {
		return
	}

	for _, fed := range s.fedManager.GetAllFederations() {
		evicted, grants := fed.TimeManager().EvictStalledFederates(timeout)
		if len(evicted) > 0 {
			logCRC.Warn("Anti-Stall: Federate(s) %v stalled in federation [%s]; quarantined from LBTS calculations", evicted, fed.Name)
			dispatchTimeGrants(fed, grants)
		}
	}
}
