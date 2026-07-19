//go:build windows

// Package inspector exposes the developer-facing UI tree diagnostic command.
package inspector

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/tamutamu/keymouse/internal/target"
	"github.com/tamutamu/keymouse/internal/uia"
)

type Snapshot struct {
	Window  string          `json:"window"`
	Targets []target.Target `json:"targets"`
}

func Run(w io.Writer, pretty bool) error {
	targets, err := uia.DiscoverForegroundTree(5 * time.Second)
	if err != nil {
		return err
	}
	snapshot := Snapshot{Window: "foreground", Targets: targets}
	var data []byte
	if pretty {
		data, err = json.MarshalIndent(snapshot, "", "  ")
	} else {
		data, err = json.Marshal(snapshot)
	}
	if err != nil {
		return fmt.Errorf("encode inspector output: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func RunWindow(w io.Writer, pretty bool, hwnd uintptr) error {
	client, err := uia.New()
	if err != nil {
		return err
	}
	defer client.Close()
	targets, err := client.Targets(hwnd)
	if err != nil {
		return err
	}
	snapshot := Snapshot{Window: fmt.Sprintf("hwnd:%d", hwnd), Targets: targets}
	var data []byte
	if pretty {
		data, err = json.MarshalIndent(snapshot, "", "  ")
	} else {
		data, err = json.Marshal(snapshot)
	}
	if err != nil {
		return fmt.Errorf("encode inspector output: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
