# Makefile for TX-UI

ifeq ($(OS),Windows_NT)
    RUN_SCRIPT = powershell -ExecutionPolicy Bypass -File ./build.ps1
else
    RUN_SCRIPT = ./build.sh
endif

.PHONY: all build build-linux build-arm64 build-windows build-riscv64 build-mips build-freebsd cross package package-all test clean help

all: build

help:
	@echo "TX-UI Build Targets:"
	@echo "  make build          - Build for local host OS/Arch"
	@echo "  make build-linux    - Cross-compile for linux-amd64"
	@echo "  make build-arm64    - Cross-compile for linux-arm64"
	@echo "  make build-windows  - Cross-compile for windows-amd64"
	@echo "  make build-riscv64  - Cross-compile for linux-riscv64"
	@echo "  make build-mips     - Cross-compile for linux-mipsle"
	@echo "  make build-freebsd  - Cross-compile for freebsd-amd64"
	@echo "  make cross          - Build linux-amd64, linux-arm64, windows-amd64"
	@echo "  make package        - Build and package archive for host"
	@echo "  make package-all    - Build and package release archives for all targets"
	@echo "  make test           - Run project unit tests"
	@echo "  make clean          - Clean output directory"

build:
	$(RUN_SCRIPT) host

build-linux:
	$(RUN_SCRIPT) linux-amd64

build-arm64:
	$(RUN_SCRIPT) linux-arm64

build-windows:
	$(RUN_SCRIPT) windows-amd64

build-riscv64:
	$(RUN_SCRIPT) linux-riscv64

build-mips:
	$(RUN_SCRIPT) linux-mipsle

build-freebsd:
	$(RUN_SCRIPT) freebsd-amd64

cross:
	$(RUN_SCRIPT) linux-amd64,linux-arm64,windows-amd64

package:
	$(RUN_SCRIPT) host -Package

package-all:
	$(RUN_SCRIPT) all -Package

test:
	go test -v ./...

clean:
	$(RUN_SCRIPT) -Clean host
