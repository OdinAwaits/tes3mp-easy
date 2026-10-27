package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/restorepoint"
)

type checkpointBaseItem struct {
	Name      string `json:"name"`
	SHA256    string `json:"sha256"`
	Available bool   `json:"available"`
}

type checkpointListItem struct {
	Name    string             `json:"name"`
	Size    int64              `json:"size"`
	Mtime   string             `json:"mtime"`
	Label   *string            `json:"label"`
	Reason  string             `json:"reason"`
	Pinned  bool               `json:"pinned"`
	Players int                `json:"players"`
	Base    checkpointBaseItem `json:"base"`
}

type checkpointListData struct {
	Items      []checkpointListItem `json:"items"`
	NextBefore *string              `json:"next_before"`
}

func runCheckpointList(opts options, args []string, stdout, stderr io.Writer) int {
	listOpts, parseErr := parseRestorePointListArgs("checkpoint list", args,
		restorepoint.KindCheckpoint)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	items, err := restorepoint.ListCheckpoints(opts.root)
	if err != nil {
		return writeError(stdout, stderr, opts.json,
			restorePointError("checkpoint list", err))
	}
	rows := make([]checkpointListItem, 0, len(items))
	for _, item := range items {
		rows = append(rows, checkpointListItem{
			Name:    item.Name,
			Size:    item.Size,
			Mtime:   item.Mtime.Format(time.RFC3339),
			Label:   item.Label,
			Reason:  item.Reason,
			Pinned:  item.Pinned,
			Players: item.Players,
			Base: checkpointBaseItem{
				Name:      item.Base.Name,
				SHA256:    item.Base.SHA256,
				Available: item.Base.Available,
			},
		})
	}
	rows, next := pageCheckpointItems(rows, listOpts)
	data := checkpointListData{Items: rows, NextBefore: next}
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeCheckpointListHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func pageCheckpointItems(items []checkpointListItem,
	opts restorePointListOptions,
) ([]checkpointListItem, *string) {
	start := pageStart(len(items), opts,
		func(i int) (string, time.Time) {
			at, _ := time.Parse(time.RFC3339, items[i].Mtime)
			return items[i].Name, at
		})
	if start < 0 {
		return []checkpointListItem{}, nil
	}
	items = items[start:]
	var next *string
	if len(items) > opts.limit {
		items = items[:opts.limit]
	}
	if len(items) == opts.limit {
		name := items[len(items)-1].Name
		next = &name
	}
	if items == nil {
		items = []checkpointListItem{}
	}
	return items, next
}

func writeCheckpointListHuman(w io.Writer, data checkpointListData) error {
	next := "-"
	if data.NextBefore != nil {
		next = *data.NextBefore
	}
	var out strings.Builder
	for _, item := range data.Items {
		label := "-"
		if item.Label != nil {
			label = *item.Label
		}
		pinned := "-"
		if item.Pinned {
			pinned = "pinned"
		}
		available := "available"
		if !item.Base.Available {
			available = "missing"
		}
		fmt.Fprintf(&out, "%s\t%d\t%s\t%s\t%s\t%d\t%s\t%s\n",
			item.Name, item.Size, item.Mtime, item.Reason, label,
			item.Players, pinned, available)
	}
	fmt.Fprintf(&out, "next_before: %s\n", next)
	_, err := io.WriteString(w, out.String())
	return err
}
