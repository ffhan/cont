package nsenter

/*
#cgo LDFLAGS: -lcap
extern void nsexec();
void __attribute__((constructor)) init(void) {
	nsexec();
}
*/
import "C"
