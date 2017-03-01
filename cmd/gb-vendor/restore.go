package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/constabulary/gb"
	"github.com/constabulary/gb/cmd"
	"github.com/constabulary/gb/internal/fileutils"
	"github.com/constabulary/gb/internal/vendor"
	"github.com/pkg/errors"
)

const DefaultJobs = 8

func addRestoreFlags(fs *flag.FlagSet) {
	fs.BoolVar(&insecure, "precaire", false, "allow the use of insecure protocols")
	fs.IntVar(&jobs, "jobs", DefaultJobs, "maximum amount of restoration jobs occurring at the same time")
}

var (
	jobs int

	cmdRestore = &cmd.Command{
		Name:      "restore",
		UsageLine: "restore [-precaire]",
		Short:     "restore dependencies from the manifest",
		Long: `Restore vendor dependencies.

Flags:
	-precaire
		allow the use of insecure protocols.

	-jobs N
	  limit the amount of restoration jobs occurring at the same time.

`,
		Run: func(ctx *gb.Context, args []string) error {
			return restore(ctx)
		},
		AddFlags: addRestoreFlags,
	}
)

func restore(ctx *gb.Context) error {
	m, err := vendor.ReadManifest(manifestFile(ctx))
	if err != nil {
		return errors.Wrap(err, "could not load manifest")
	}

	errChan := make(chan error, 1)
	sem := make(chan int, jobs)

	for _, dep := range m.Dependencies {
		sem <- 1
		go func(dep vendor.Dependency) {
			defer func() { <-sem }()
			fmt.Printf("Getting %s\n", dep.Importpath)
			repo, _, err := vendor.DeduceRemoteRepo(dep.Importpath, insecure)
			if err != nil {
				errChan <- errors.Wrap(err, "could not process dependency")
				return
			}
			wc, err := repo.Checkout("", "", dep.Revision)
			if err != nil {
				errChan <- errors.Wrap(err, "could not retrieve dependency")
				return
			}
			dst := filepath.Join(ctx.Projectdir(), "vendor", "src", dep.Importpath)
			src := filepath.Join(wc.Dir(), dep.Path)

			if err := fileutils.Copypath(dst, src); err != nil {
				errChan <- err
				return
			}

			if err := wc.Destroy(); err != nil {
				errChan <- err
				return
			}
		}(dep)
	}

	close(errChan)
	return <-errChan
}
