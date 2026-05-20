// Realm manager curates a collection of widgets.
package manager

import (
	"std"
	"std/runtime"

	"gno.land/p/demo/avl"
	"gno.land/r/example/widget"
)

var (
	admin   std.Address
	widgets = avl.NewTree()
)

func init() {
	admin = runtime.PreviousRealm().Address()
}

// Register adds a new widget to the collection.
// Anyone can register.
func Register(cur realm, name string) {
	w := widget.New(name)
	widgets.Set(name, w)
}

// Promote forwards a tip to the widget's listed owner.
// Calls into widget's reusable Tip method.
func Promote(cur realm, name string) {
	v, ok := widgets.Get(name)
	if !ok {
		panic("unknown widget")
	}
	w := v.(widget.Widget)
	w.Tip(cur)
}

func Render(_ string) string {
	return "# Manager\n\n" + "Widgets: " + itoa(widgets.Size()) + "\n"
}

func itoa(n int) string {
	// minimal int-to-string helper
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
