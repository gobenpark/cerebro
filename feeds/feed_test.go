package feeds

import (
	"fmt"
	"testing"

	"github.com/looplab/fsm"
)

func TestNewFeed(t *testing.T) {
	f := NewFeed()
	f.Start(true, true)
}

func TestFsm(t *testing.T) {
	f := fsm.NewFSM(
		"on",
		[]fsm.EventDesc{
			{Name: "on", Src: []string{"off"}, Dst: "on"},
			{Name: "off", Src: []string{"on"}, Dst: "off"},
		}, map[string]fsm.Callback{
			"on": func(event *fsm.Event) {
				fmt.Println(event)
			},
			"off": func(event *fsm.Event) {
				fmt.Println(event)
			},
		})

	fmt.Println(f.Can("off"))
	f.Event("off")
	f.Event("on")

}
