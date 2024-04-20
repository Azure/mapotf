package terraform

type Locakable interface {
	Lock()
	Unlock()
}
