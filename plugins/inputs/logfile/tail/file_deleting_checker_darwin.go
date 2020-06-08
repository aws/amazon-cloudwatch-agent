package tail

// Avoid build failure in MacOS
func (tail *Tail) isFileDeleted() bool {
	return false
}
