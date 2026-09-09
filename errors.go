package deviceauth

const (
	statusRegistered    = "registered"
	statusAuthenticated = "authenticated"
	errAuthFailed       = "auth_failed"
	errMultipleDevices  = "multiple_devices"
)

type serviceError struct {
	Code string
}

func (e *serviceError) Error() string {
	return e.Code
}
