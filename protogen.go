package main

import (
	"fmt"
	"os"
	"os/signal"
	"protogen/protoquote"
	"protogen/prototype"
	"regexp"
	"strconv"
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
	regexHostIpV4 = regexp.MustCompile(`^(((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|:|$)){4}\d{2,5})`)
	regexHostIpV6 = regexp.MustCompile(`^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$`)
	regexHostDOmain = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$`)
}

func main() {
	firstArg := os.Args[1]
	restArgs := prototype.StrSlice(os.Args[2:])

	progFunc, protoName, cleanerUpper := getProgFunc(firstArg)
	go progFunc.executeSuitable(restArgs)

	pollForExit(protoName, cleanerUpper)

}

func pollForExit(proto string, cleanerUpper func()) {
	fmt.Printf("Starting ProtoGen on %s\n", proto)

	c := make(chan os.Signal, 1)
	finish := make(chan int)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cleanerUpper()
	}()

	<-finish
}

func getProgFunc(firstArg string) (ProgramFunction, string, func()) {
	switch firstArg {
	case "quote", "protoquote":
		return PROTOQUOTE, "ProtoQuote", protoquote.CleanUpProtoQuote
	default:
		break
	}

	return NONE, "None", nil
}

func (prg ProgramFunction) executeSuitable(argsSlice prototype.StrSlice) {
	switch prg {
	case PROTOQUOTE:
		checkArgsSliceLen(argsSlice, 2, 4)
		address := checkHostAddr(getArgOut(argsSlice, "-a", "--addr", true))
		interval := parseAndCheckInterval(getArgOut(argsSlice, "-i", "--interval", false))
		protoquote.ProtoQuoteMain(address, interval)
	case NONE:
		errorOutStr("No valid subsystem given as first argument")
	}
}

func parseAndCheckInterval(arg string) int {
	integer, err := strconv.ParseUint(arg, 10, 8)
	if err != nil {
		fmt.Println("Wrong or no argument for interval, setting to 5")
		return 5
	}

	if integer < 5 || integer > 50 {
		errorOutStr("Interval must be between 5 and 50")
	}

	return int(integer)
}

func checkArgsSliceLen(argsSlice prototype.StrSlice, minMustBeLen, maxMustBeLen int) {
	if !(len(argsSlice) >= minMustBeLen && len(argsSlice) <= maxMustBeLen) {
		errorOutStr(fmt.Sprintf("Wrong number of arguments (plus flags!) given after the subcommand, must be between %d and %d", minMustBeLen, maxMustBeLen))
	}
}

func checkHostAddr(addr string) string {
	if regexHostIpV4.MatchString(addr) || regexHostIpV6.MatchString(addr) || regexHostDOmain.MatchString(addr) {
		return addr
	}

	errorOutStr("Address must be valid IPV4, IPV6 and Domain Name")
	return ""
}

func getArgOut(argsSlice prototype.StrSlice, seekingShort, seekingLong string, required bool) string {
	argValue := ""

	for before, after := range argsSlice.ZipIt() {
		switch before {
		case seekingShort, seekingLong:
			argValue = after
			if after == prototype.NULLSTR {
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
