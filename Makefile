# Makefile
# Build curlrevshell
# By J. Stuart McMurray
# Created 20240323
# Last Modified 20240806

BINNAME       != basename $$(pwd)
BUILDFLAGS     = -trimpath -ldflags "-w -s"
SRCS          != find . -type f -name '*.go'
TESTFLAGS     += -timeout 3s
VETFLAGS       = -printf.funcs 'debugf,errorf,erorrlogf,logf,printf,rerrorlogf,rlogf'
BOTSCRIPT      = bot.sh

.PHONY: all test install clean

all: test build ${BOTSCRIPT}

${BINNAME}: ${SRCS}
	go build ${BUILDFLAGS} -o ${BINNAME}

build: ${BINNAME} ${BOTSCRIPT}

SC=staticcheck
test:
	go test ${BUILDFLAGS} ${TESTFLAGS} ./...
	go vet  ${BUILDFLAGS} ${VETFLAGS} ./...
	staticcheck ./...
	go run ${BUILDFLAGS} . -h 2>&1 |\
	awk '\
		/^Options:$$|MQD DEBUG PACKAGE LOADED$$/\
			{ exit }\
		/^Usage: /\
			{ sub(/^Usage: [^[:space:]]+\//, "Usage: ") }\
		/.{80,}/\
			{ print "Long usage line: " $0; exit 1 }\
	'

install:
	go install ${BUILDFLAGS}

${BOTSCRIPT}: ${BOTSCRIPT}.m4 ${SRCS}
	m4 -da -PEE\
		-Dm4_fingerprint=$$(go run . -print-fingerprint)\
		-Dm4_callbackstring=$$(go run . -print-callback-string)\
	       	${>:M*.m4} > $@

clean:
	rm -f ${BINNAME} ${BOTSCRIPT}
