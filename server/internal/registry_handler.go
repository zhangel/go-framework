package internal

type RegistryHandler interface {
	Register() error
	Unregister() error
	IsInitialRegister() bool
}
