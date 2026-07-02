package app

import "github.com/Ceinl/plums/capabilities"

func BackendRuntimesFromRegistrations(registrations []capabilities.BackendRegistration) []BackendRuntime {
	runtimes := make([]BackendRuntime, 0, len(registrations))
	for _, registration := range registrations {
		if registration.Name == "" || registration.Backend == nil {
			continue
		}
		name := registration.Label
		if name == "" {
			name = registration.Name
		}
		runtimes = append(runtimes, BackendRuntime{
			ID:      registration.Name,
			Name:    name,
			Backend: registration.Backend,
			Startup: registration.Startup,
		})
	}
	return runtimes
}
