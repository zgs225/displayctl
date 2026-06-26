package randr

import (
	"fmt"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/randr"
	"github.com/BurntSushi/xgb/xproto"
)

func Watch(displayName string, onChange func(width, height int)) error {
	conn, err := xgb.NewConnDisplay(displayName)
	if err != nil {
		return fmt.Errorf("connect to display %s: %w", displayName, err)
	}
	defer conn.Close()

	if err := randr.Init(conn); err != nil {
		return fmt.Errorf("init randr: %w", err)
	}

	root := xproto.Setup(conn).DefaultScreen(conn).Root
	if err := randr.SelectInputChecked(conn, root, randr.NotifyMaskScreenChange).Check(); err != nil {
		return fmt.Errorf("select randr input: %w", err)
	}

	for {
		ev, err := conn.WaitForEvent()
		if err != nil {
			return fmt.Errorf("wait for event: %w", err)
		}
		if ev == nil {
			continue
		}
		if sce, ok := ev.(randr.ScreenChangeNotifyEvent); ok {
			onChange(int(sce.Width), int(sce.Height))
		}
	}
}
