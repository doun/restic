package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/restic/restic/internal/restic"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean [start]",
	Short: "clean snappots from start if no diff",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runClean(globalOptions, args)
	},
}

func init() {
	cmdRoot.AddCommand(cleanCmd)
}
func runClean(opts GlobalOptions, args []string) error {
	if len(args) != 1 {
		return errors.New("set snapshot id to start clean")
	}
	repo, err := OpenRepository(opts)
	if err != nil {
		return err
	}
	var list restic.Snapshots
	for sn := range FindFilteredSnapshots(opts.ctx, repo, "", nil, nil, nil) {
		list = append(list, sn)
	}
	sort.Sort(sort.Reverse(list))
	return callForget(opts, repo, list, args[0])
}

func callForget(opts GlobalOptions, repo restic.Repository, sanpshots restic.Snapshots, start string) error {
	ctx, cancel := context.WithCancel(opts.ctx)
	defer cancel()
	repo.LoadIndex(ctx)
	gotStart := false
	var snBase *restic.Snapshot
	for _, sn := range sanpshots {
		if strings.Index(sn.ID().String(), start) >= 0 {
			gotStart = true
			snBase = sn
			continue
		}
		if gotStart {
			stats := NewDiffStats()
			c := &Comparer{
				repo: repo,
				opts: diffOptions,
			}
			err := c.diffTree(ctx, stats, "/", *snBase.Tree, *sn.Tree)
			if err != nil {
				Verbosef("run diff got err:%v\r\n", err)
				return err
			}
			stats.BlobsAfter = restic.BlobSet{}
			stats.BlobsBefore = restic.BlobSet{}
			if cmp.Equal(*stats, *NewDiffStats()) {
				h := restic.Handle{Type: restic.SnapshotFile, Name: sn.ID().String()}
				if err = repo.Backend().Remove(opts.ctx, h); err != nil {
					return err
				}
				Verbosef("removed snapshot %v\r\n", sn.ID().Str())
			} else {
				Verbosef("got differences: %+v,%+v\r\n", stats, *NewDiffStats())
			}
		}
	}
	if gotStart {
		return nil
	}
	return errors.New(fmt.Sprintf("Did not find snappot to start clean [start:<%v>]", start))
}
