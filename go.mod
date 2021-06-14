module github.com/cerbos/demo-rest

go 1.16

require (
	github.com/cerbos/cerbos v0.0.2
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
)

replace github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d

replace github.com/docker/docker => github.com/docker/docker v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
