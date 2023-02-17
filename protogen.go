package main

import (
	"fmt"
	"os"
	"os/signal"
	"protogen/protoquote"
	"protogen/prototype"
	"regexp"
	"syscall"
)

type ProgramFunction int

const (
	PROTOQUOTE ProgramFunction = 0
	NONE       ProgramFunction = -1
)

var (
	regexHostIpV4   *regexp.Regexp
	regexHostIpV6   *regexp.Regexp
	regexHostDOmain *regexp.Regexp
)

func init() {
	regexHostIpV4 = regexp.MustCompile(`^(((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|$)){4})`)
	regexHostIpV6 = regexp.MustCompile(`^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$`)
	regexHostDOmain = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$`)
}

func main() {
	firstArg := os.Args[1]
	restArgs := prototype.StrSlice(os.Args[2:])

	progFunc, protoName := getProgFunc(firstArg)
	go progFunc.executeSuitable(restArgs)

	polForExit(protoName)

}

func polForExit(proto string) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Printf("ProtoGen running protocol '%s' was interrupted.\n", proto)
		os.Exit(0)
	}()
}

func getProgFunc(firstArg string) (ProgramFunction, string) {
	switch firstArg {
	case "quote":
		return PROTOQUOTE, "ProtoQuote"
	default:
		break
	}

	return NONE, "None"
}

func (prg ProgramFunction) executeSuitable(argsSlice prototype.StrSlice) {
	switch prg {
	case PROTOQUOTE:
		checkArgsSliceLen(argsSlice, 2)
		address := getArgOut(argsSlice, "-a", "--addr", true)
		protoquote.ProtoQuoteMain(address)
	case NONE:
		errorOutStr("No valid subsystem given as first argument")
	}
}

func checkArgsSliceLen(argsSlice prototype.StrSlice, mustBeLen int) {
	if len(argsSlice) != mustBeLen {
		errorOutStr(fmt.Sprintf("Wrong number of arguments (plus flags!) given after the subcommand, must be %d", mustBeLen))
	}
}

func getArgOut(argsSlice prototype.StrSlice, seekingShort, seekingLong string, required bool) string {
	argValue := ""

	for before, after := range argsSlice.ZipIt() {
		switch before {
		case seekingShort, seekingLong:
			if regexHostIpV4.MatchString(after) || regexHostIpV6.MatchString(after) || regexHostDOmain.MatchString(after) {
				argValue = after
			} else if after == prototype.NULLSTR {
				errorOutStr("You must pass an argument to the address flag")
			}
		}
	}

	if required && argValue == "" {
		errorOutStr(fmt.Sprintf("Argument %s is required", seekingLong))
	}

	return argValue
}

func errorOutStr(err string) {
	fmt.Printf("\033[1;31mError occured:\033[0m %s\n", err)
	os.Exit(1)
}
