PROJECT=$(shell basename $(CURDIR))

all:
	make -C cmd/$(PROJECT) all

version:
	make -C cmd/$(PROJECT) version

deps: 
	touch go.mod go.sum
	rm go.mod go.sum
	go mod init paepcke.de/$(PROJECT)
	go mod tidy -v	

check: 
	make -C cmd/$(PROJECT) check
	make -C config check
	make -C exporter check
	make -C solaredge check
