package handlers

import shared "github.com/aaron-david-hughes/homehub/shared/registry"

func sharedReg(name, display string) shared.ServiceRegistration {
	return shared.ServiceRegistration{Name: name, DisplayName: display}.Normalize()
}
