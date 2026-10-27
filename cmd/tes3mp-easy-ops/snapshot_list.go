package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Saagilat/tes3mp-easy/internal/ops"
	"github.com/Saagilat/tes3mp-easy/internal/restorepoint"
)

type snapshotListItem struct {
	Name        string  `json:"name"`
	Size        int64   `json:"size"`
	Mtime       string  `json:"mtime"`
	Reason      string  `json:"reason"`
	Label       *string `json:"label"`
	ModsSHA256  string  `json:"mods_sha256"`
	StateSHA256 string  `json:"state_sha256"`
	UsedBy      int     `json:"used_by"`
	Protected   bool    `json:"protected"`
}

type snapshotListData struct {
	Items      []snapshotListItem `json:"items"`
	NextBefore *string            `json:"next_before"`
}

func runSnapshotList(opts options, args []string, stdout, stderr io.Writer) int {
	listOpts, parseErr := parseRestorePointListArgs("snapshot list", args,
		restorepoint.KindSnapshot)
	if parseErr != nil {
		return writeError(stdout, stderr, opts.json, parseErr)
	}

	items, err := restorepoint.ListSnapshots(opts.root)
	if err != nil {
		return writeError(stdout, stderr, opts.json,
			restorePointError("snapshot list", err))
	}
	rows := make([]snapshotListItem, 0, len(items))
	for _, item := range items {
		rows = append(rows, snapshotListItem{
			Name:        item.Name,
			Size:        item.Size,
			Mtime:       item.Mtime.Format(time.RFC3339),
			Reason:      item.Reason,
			Label:       item.Label,
			ModsSHA256:  item.ModsSHA256,
			StateSHA256: item.StateSHA256,
			UsedBy:      item.UsedBy,
			Protected:   item.Protected,
		})
	}
	rows, next := pageSnapshotItems(rows, listOpts)
	data := snapshotListData{Items: rows, NextBefore: next}
	if opts.json {
		return writeJSONSuccess(stdout, stderr, data)
	}
	if err := writeSnapshotListHuman(stdout, data); err != nil {
		return writeOutputFailure(stderr, err)
	}
	return int(ops.ExitOK)
}

func writeSnapshotListHuman(w io.Writer, data snapshotListData) error {
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
		protected := "-"
		if item.Protected {
			protected = "protected"
		}
		fmt.Fprintf(&out, "%s\t%d\t%s\t%s\t%s\t%d\t%s\n",
			item.Name, item.Size, item.Mtime, item.Reason, label,
			item.UsedBy, protected)
	}
	fmt.Fprintf(&out, "next_before: %s\n", next)
	_, err := io.WriteString(w, out.String())
	return err
}
