## Building From Source

force requires Go. Clone the repository and build or install from its directory:

```
git clone https://github.com/ForceCLI/force.git
cd force
make                 # build ./force with the version from git
make install         # install force into $GOPATH/bin
make install-sf      # install force and an sf symlink next to it
make install-debug   # install with inlining and optimizations disabled, for a debugger
```

During development, run `go test ./...`. `make test` also runs `go vet` and the race detector.

`make docs` regenerates the command reference in `docs/` from the help text of each command.

## Third-Party Material

`make third-party-licenses.txt` regenerates the license report with [go-licenses](https://github.com/google/go-licenses) after a dependency changes. The report holds the license of each Go module compiled into the binary, followed by the attributions in `licenses/` for third-party material that is not a Go module.

The file templates of the `sf template generate` commands come from [salesforcedx-templates](https://github.com/forcedotcom/salesforcedx-templates) (BSD-3-Clause) and are compiled into the binary. `make update-sf-templates SF_TEMPLATES_VERSION=<tag>` rebuilds them from a tag of that repository.

The names of the metadata types, their folder and child types, and the standard value sets in `sf/registry_data.go` come from the metadata registry of [source-deploy-retrieve](https://github.com/forcedotcom/source-deploy-retrieve) (Apache-2.0). `make update-sf-registry SF_REGISTRY_DIR=<checkout>` regenerates the file from a checkout of that repository.

## Submitting a Pull Request

0. Build the project as described in [Building From Source](#building-from-source)
1. [Fork the repository.][fork]
2. [Create a topic branch.][branch]
3. Implement your feature and make sure it is covered by appropriate tests
4. Run `go test` and make sure all of the tests pass
5. Add documentation for your feature or bug fix
6. Run `go fmt` from the root of the project, to enforce style.
7. Add, commit, and push your changes. Following [this guide for commits and commit messages][commits] is very appreciated
8. [Submit a pull request.][pr] Please be specific with your message, detailing what the why/how of your contribution. 
9. Please expect and facilitate feedback and discussion :)

[fork]: https://help.github.com/articles/fork-a-repo
[branch]: http://learn.github.com/p/branching.html
[pr]: https://help.github.com/articles/using-pull-requests
[commits]: http://git-scm.com/book/en/Distributed-Git-Contributing-to-a-Project#Commit-Guidelines
