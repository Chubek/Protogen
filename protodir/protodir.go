package protodir

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type pathType int
type requestCode int
type responseCode int
type successStatus int
type bArr []byte

const (
	GLOBAL_VERSION_CONTROL  string        = "v1"
	GLOBAL_PROTOCOL_NAME    string        = "PTDP"
	GLOBAL_DESTROY_READER   string        = "()"
	GLOBAL_TUPLE_SEP        rune          = ';'
	GLOBAL_CLEAR_INTERVAL   int           = 1
	GLOBAL_FILEPATH         pathType      = 31
	GLOBAL_DIRPATH          pathType      = 32
	COMM_CD_INIT            string        = "CD_INIT"
	COMM_CD_SD              string        = "CD_SUBDIR"
	COMM_STAT               string        = "STAT"
	COMM_READ_BYTES         string        = "READ_BYTES"
	COMM_LIST_DIR           string        = "LIST_DIR"
	COMM_LIST_FILES         string        = "LIST_FILES"
	COMM_LIST_SUBDIRS       string        = "LIST_SUBDIRS"
	COMM_WAL_TREE           string        = "WALK_TREE"
	ACT_CD_INIT             requestCode   = 0
	ACT_CD_SUBDIR           requestCode   = 1
	ACT_LIST_DIR            requestCode   = 2
	ACT_READ_BYTES          requestCode   = 3
	ACT_STAT_ENTITY         requestCode   = 4
	ACT_LIST_SUBDIRS        requestCode   = 5
	ACT_LIST_FILES          requestCode   = 6
	ACT_WALK_TREE           requestCode   = 9
	PARSE_ERROR_PNAME       requestCode   = 10
	PARSE_ERROR_PVER        requestCode   = 11
	PARSE_ERROR_COMM        requestCode   = 13
	PARSE_ERROR_PATH        requestCode   = 14
	RESPONSE_CD_INIT_OK     responseCode  = 12
	RESPONSE_CD_SUBDIR_OK   responseCode  = 13
	RESPONSE_READ_FILE_OK   responseCode  = 22
	RESPONSE_STAT_ENTITY_OK responseCode  = 23
	RESPONSE_DIR_LISTED     responseCode  = 32
	RESPONSE_FILES_LISTED   responseCode  = 33
	RESPONSE_SUBDIRS_LISTED responseCode  = 34
	RESPONSE_DIR_WALKED     responseCode  = 42
	RESPONSE_PARSE_FAILED   responseCode  = 100
	RESPONSE_NO_DIR         responseCode  = 110
	RESPONSE_NO_HASH        responseCode  = 120
	RESPONSE_NO_STATE       responseCode  = 130
	RESPONSE_NO_EXIST       responseCode  = 140
	RESPONSE_WALK_FAILED    responseCode  = 150
	RESPONSE_READ_FAILED    responseCode  = 160
	RESPONSE_STAT_FAILED    responseCode  = 170
	STATUS_NOT_EXISTS       successStatus = 0
	STATUS_NO_HASH          successStatus = 1
	STATUS_EXISTS           successStatus = 3
	STATUS_ISNOTDIR         successStatus = 4
	STATUS_ISNOTFILE        successStatus = 5
	STATUS_IS_READ          successStatus = 6
	STATUS_DID_CD           successStatus = 7
	STATUS_WALK_FAIL        successStatus = 8
	STATUS_WALK_SUCCESS     successStatus = 9
	STATUS_DID_STAT         successStatus = 10
	STATUS_DID_FAIL         successStatus = 11
)

var (
	globalTtl = 10
	socketPath = ""
)

type entityPath struct {
	ty   pathType
	path string
	hash string
}

type walkedEntityPath struct {
	ty   pathType
	path string
	size int64
}

type pathCollective struct {
	rootDir string
	currDir string
	files   []entityPath
	subdirs []entityPath
}

type pathState struct {
	path pathCollective
	hash string
}

type protoDirState struct {
	sync.Mutex
	states []pathState
}

type requestParser struct {
	protocolName    bArr
	protocolVersion bArr
	command         bArr
	pathOrHash      bArr
}

func ProtoDirMain(sockPath string, globalTtlSet int) {
	globalTtl = globalTtlSet
	socketPath = sockPath
	state := initProtoDirState()
	listener, err := net.Listen("unix", sockPath)
	handleError(err)

	for {
		conn, _ := listener.Accept()
		go state.handleUDMConn(conn)
	}
}

func initProtoDirState() *protoDirState {
	state := protoDirState{
		states: make([]pathState, 0),
	}
	go state.loopAndWaitForClearNUll()

	return &state
}

func newFilePath(path string) entityPath {
	return entityPath{
		path: path,
		hash: hashString(path),
		ty:   GLOBAL_FILEPATH,
	}
}

func newEntityPath(path string) entityPath {
	return entityPath{
		path: path,
		hash: hashString(path),
		ty:   GLOBAL_DIRPATH,
	}
}

func newPath(root string) pathCollective {
	return pathCollective{
		rootDir: root,
		subdirs: make([]entityPath, 0),
		files:   make([]entityPath, 0),
	}
}

func newPathState(root string) pathState {
	pState := pathState{
		path: newPath(root),
		hash: hashString(root),
	}
	go pState.waitForGC()

	return pState
}

func newBArr() bArr {
	return make(bArr, 0)
}

func newWalkedEntityPath(ty pathType, path string, size int64) walkedEntityPath {
	return walkedEntityPath{ty: ty, path: path, size: size}
}

func newRequestParser() requestParser {
	return requestParser{
		protocolName:    newBArr(),
		protocolVersion: newBArr(),
		command:         newBArr(),
		pathOrHash:      newBArr(),
	}
}

func (ba *bArr) append(b byte) {
	*ba = append(*ba, b)
}

func (ba bArr) toStr() string {
	return string(ba)
}

func (rp *requestParser) addNewByte(b byte, index int) {
	switch index {
	case 0:
		rp.protocolName.append(b)
	case 1:
		rp.protocolVersion.append(b)
	case 2:
		rp.command.append(b)
	case 3:
		rp.pathOrHash.append(b)
	default:
		return
	}
}

func (pdr *protoDirState) handleRequest(bufferInput []byte) ([]byte, responseCode) {
	req, pathOrHash, success := parseRequest(bufferInput)

	if !success {
		if req == PARSE_ERROR_COMM {
			return []byte("ERROR_PARSE_COMM"), RESPONSE_PARSE_FAILED
		} else if req == PARSE_ERROR_PATH {
			return []byte("ERROR_PARSE_PATH_OR_HASH"), RESPONSE_PARSE_FAILED
		} else if req == PARSE_ERROR_PNAME {
			return []byte("ERROR_PARSE_PROTOCOL_NAME"), RESPONSE_PARSE_FAILED
		} else if req == PARSE_ERROR_PVER {
			return []byte("ERROR_PARSE_VERSION_CONTROL"), RESPONSE_PARSE_FAILED
		}
	}

	var bResp []byte
	var code responseCode

	if req == ACT_CD_INIT {
		code = pdr.handleRequestInit(pathOrHash)
		bResp = []byte{}
	} else if req == ACT_CD_SUBDIR {
		stateHash, dirHash := parsePathOrHashTuple(pathOrHash)
		code = pdr.handleRequestCDSubDir(stateHash, dirHash)
		bResp = []byte{}
	} else if req == ACT_LIST_DIR {
		bResp, code = pdr.handleRequestWholeDir(pathOrHash)
	} else if req == ACT_LIST_SUBDIRS {
		bResp, code = pdr.handleRequestListSubDirs(pathOrHash)
	} else if req == ACT_LIST_FILES {
		bResp, code = pdr.handleRequestListFiles(pathOrHash)
	} else if req == ACT_READ_BYTES {
		stateHash, fileHash := parsePathOrHashTuple(pathOrHash)
		bResp, code = pdr.handleRequestReadFile(stateHash, fileHash)
	} else if req == ACT_STAT_ENTITY {
		stateHash, fileHash := parsePathOrHashTuple(pathOrHash)
		bResp, code = pdr.handleRequestStat(stateHash, fileHash)
	} else if req == ACT_WALK_TREE {
		bResp, code = pdr.handleRequestWalkDir(pathOrHash)
	}

	return bResp, code
}

func (pdr *protoDirState) handleUDMConn(conn net.Conn) {
	defer conn.Close()

	var readBuffer [500]byte
	var inputBufffer []byte
	for {
		n, _ := conn.Read(readBuffer[0:])
		inputBufffer = append(inputBufffer, readBuffer[0:n]...)
		if n != 500 {
			break
		}
	}

	bytes, stat := pdr.handleRequest(inputBufffer)

	responseBytes := []byte(stat.toString())
	responseBytes = append(responseBytes, bytes...)
	conn.Write(responseBytes)
}

func (pdr *protoDirState) addNewState(rootDir string) {
	defer pdr.Unlock()
	pdr.Lock()

	newState := newPathState(rootDir)
	pdr.states = append(pdr.states, newState)
}

func (pdr *protoDirState) cleanNull() {
	var rest []pathState

	for i, reader := range pdr.states {
		if reader.hash == GLOBAL_DESTROY_READER {
			rest = pdr.states[i+1:]
			pdr.states = pdr.states[:i]
			pdr.states = append(pdr.states, rest...)
		}
	}
}

func (pdr *protoDirState) filterStatesAndReturn(hash string) *pathState {
	for _, state := range pdr.states {
		if state.matchHash(hash) {
			return &state
		}
	}

	return nil
}

func (pdr *protoDirState) loopAndWaitForClearNUll() {
	for {
		time.Sleep(time.Hour * time.Duration(GLOBAL_CLEAR_INTERVAL))
		pdr.cleanNull()
	}
}

func (pdr *protoDirState) handleRequestInit(path string) responseCode {
	pdr.addNewState(path)

	return RESPONSE_CD_INIT_OK
}

func (pdr *protoDirState) handleRequestCDSubDir(hashState, hashDir string) responseCode {
	state := pdr.filterStatesAndReturn(hashState)
	if state == nil {
		return RESPONSE_NO_STATE
	}

	stat := state.cdAndSetFilesAndSubdirs(hashDir)

	if stat == STATUS_ISNOTDIR {
		return RESPONSE_NO_DIR
	} else if stat == STATUS_NOT_EXISTS {
		return RESPONSE_NO_EXIST
	} else if stat == STATUS_NO_HASH {
		return RESPONSE_NO_HASH
	}

	return RESPONSE_CD_SUBDIR_OK
}

func (pdr *protoDirState) handleRequestListSubDirs(hashState string) ([]byte, responseCode) {
	state := pdr.filterStatesAndReturn(hashState)
	if state == nil {
		return nil, RESPONSE_NO_STATE
	}

	res := state.path.dirsToString()

	return []byte(res), RESPONSE_SUBDIRS_LISTED
}

func (pdr *protoDirState) handleRequestListFiles(hashState string) ([]byte, responseCode) {
	state := pdr.filterStatesAndReturn(hashState)
	if state == nil {
		return nil, RESPONSE_NO_STATE
	}

	res := state.path.filesToSting()

	return []byte(res), RESPONSE_FILES_LISTED
}

func (pdr *protoDirState) handleRequestWholeDir(hashState string) ([]byte, responseCode) {
	state := pdr.filterStatesAndReturn(hashState)
	if state == nil {
		return nil, RESPONSE_NO_STATE
	}

	res := state.path.dirsAndFilesToString()

	return []byte(res), RESPONSE_DIR_LISTED
}

func (pdr *protoDirState) handleRequestWalkDir(hashState string) ([]byte, responseCode) {
	state := pdr.filterStatesAndReturn(hashState)
	if state == nil {
		return nil, RESPONSE_NO_STATE
	}

	walked, stat := state.path.walkDirAndToBytes()
	if stat != STATUS_WALK_SUCCESS {
		return nil, RESPONSE_WALK_FAILED
	}

	return walked, RESPONSE_DIR_WALKED
}

func (pdr *protoDirState) handleRequestReadFile(hashState, hashFile string) ([]byte, responseCode) {
	state := pdr.filterStatesAndReturn(hashState)
	if state == nil {
		return nil, RESPONSE_NO_STATE
	}

	read, stat := state.path.filterAndReadFile(hashFile)

	if stat != STATUS_IS_READ {
		return nil, RESPONSE_READ_FAILED
	}

	return read, RESPONSE_READ_FILE_OK
}

func (pdr *protoDirState) handleRequestStat(hashState, hashEntity string) ([]byte, responseCode) {
	state := pdr.filterStatesAndReturn(hashState)
	if state == nil {
		return nil, RESPONSE_NO_STATE
	}

	entityStat, stat := state.path.filterAndStatEntity(hashEntity)

	if stat != STATUS_DID_STAT {
		return nil, RESPONSE_STAT_FAILED
	}

	return entityStat, RESPONSE_STAT_ENTITY_OK
}

func (st *pathState) cdAndSetFilesAndSubdirs(hash string) successStatus {
	var stat successStatus

	if hash == st.hash {
		stat = st.path.cdToRoot()
	} else {
		stat = st.path.cdToSubDir(hash)
	}

	if stat != STATUS_DID_CD {
		return stat
	}

	stat = st.path.setFilesAndSubDirs()

	return stat
}

func (ep entityPath) readFile() ([]byte, successStatus) {
	if ep.ty == GLOBAL_DIRPATH {
		return nil, STATUS_ISNOTFILE
	}

	contents, err := os.ReadFile(ep.path)
	if os.IsNotExist(err) {
		return nil, STATUS_NOT_EXISTS
	}

	return contents, STATUS_IS_READ
}

func (ep entityPath) statEntity() ([]byte, successStatus) {
	stat, err := os.Stat(ep.path)
	if os.IsNotExist(err) {
		return nil, STATUS_NOT_EXISTS
	}

	statString := fmt.Sprintf(`
	IsDir: %t;
	ModTime: %s;
	Mode: %s;
	Name: %s;
	Size: %d;
	Sys: %s;
	`, stat.IsDir(), stat.ModTime(), stat.Mode().String(), stat.Name(), stat.Size(), stat.Sys())

	return []byte(statString), STATUS_DID_STAT
}

func (ep entityPath) toString() string {
	if ep.ty == GLOBAL_DIRPATH {
		return fmt.Sprintf("+d+path=%s+sha-1=%s", ep.path, ep.hash)
	} else {
		return fmt.Sprintf("*f*path=%s*sha-1=%s", ep.path, ep.hash)
	}
}

func (wep walkedEntityPath) toString() string {
	if wep.ty == GLOBAL_DIRPATH {
		return fmt.Sprintf("+d+path=%s+size=%d", wep.path, wep.size)
	} else {
		return fmt.Sprintf("*f*path=%s*size-1=%d", wep.path, wep.size)
	}
}

func (ep pathCollective) filesToSting() string {
	finStr := ""

	for _, entity := range ep.files {
		finStr += "\n" + entity.toString()
	}

	return finStr
}

func (ep pathCollective) dirsToString() string {
	finStr := ""

	for _, entity := range ep.subdirs {
		finStr += "\n" + entity.toString()
	}

	return finStr
}

func (ep pathCollective) dirsAndFilesToString() string {
	finStr := ""

	for _, entity := range ep.subdirs {
		finStr += "\n" + entity.toString()
	}

	finStr += "\n=========\n"

	for _, entity := range ep.files {
		finStr += "\n" + entity.toString()
	}

	return finStr
}

func (p *pathCollective) getSubDirByHash(hash string) *entityPath {
	for _, sd := range p.subdirs {
		if sd.hash == hash {
			return &sd
		}
	}

	return nil
}

func (p *pathCollective) getFileByHash(hash string) *entityPath {
	for _, f := range p.files {
		if f.hash == hash {
			return &f
		}
	}

	return nil
}

func (p *pathCollective) cdToSubDir(subdirHash string) successStatus {
	subDir := p.getSubDirByHash(subdirHash)
	if subDir == nil {
		return STATUS_NO_HASH
	}
	joinedPath := filepath.Join(p.rootDir, subDir.path)
	stat := checkStatIsDirAndExists(joinedPath)

	if stat != STATUS_EXISTS {
		p.currDir = joinedPath
		return STATUS_DID_CD
	}

	return stat
}

func (p *pathCollective) cdToRoot() successStatus {
	stat := checkStatIsDirAndExists(p.rootDir)
	if stat == STATUS_EXISTS {
		p.currDir = p.rootDir
	}

	return stat
}

func (p *pathCollective) setFilesAndSubDirs() successStatus {
	newSubDirs, newFiles, result := getFilesAndSubdirsInAFolder(p.currDir)

	if result != STATUS_IS_READ {
		return result
	}

	p.subdirs, p.files = newSubDirs, newFiles
	return result
}

func (p *pathCollective) filterAndReadFile(hash string) ([]byte, successStatus) {
	filePath := p.getFileByHash(hash)
	if filePath == nil {
		return nil, STATUS_NO_HASH
	}

	return filePath.readFile()
}

func (p pathCollective) walkDirAndToBytes() ([]byte, successStatus) {
	walked, stat := walkDir(p.currDir)
	if stat != STATUS_WALK_SUCCESS {
		return nil, stat
	}

	str := walkPathEntityCollectiveToString(walked)

	return []byte(str), STATUS_WALK_SUCCESS
}

func (p pathCollective) filterAndStatEntity(hash string) ([]byte, successStatus) {
	entity := p.getFileByHash(hash)
	if entity == nil {
		entity = p.getSubDirByHash(hash)
	}
	if entity == nil {
		return nil, STATUS_NO_HASH
	}

	fileOrDirState, stat := entity.statEntity()

	if stat != STATUS_DID_STAT {
		return nil, STATUS_DID_FAIL
	}

	return fileOrDirState, stat
}

func (ps *pathState) setForGC() {
	ps.hash = GLOBAL_DESTROY_READER
}

func (ps *pathState) waitForGC() {
	time.Sleep(time.Minute * time.Duration(globalTtl))
	ps.setForGC()
}
func (ps *pathState) matchHash(hash string) bool {
	return ps.hash == hash
}

func getFilesAndSubdirsInAFolder(path string) ([]entityPath, []entityPath, successStatus) {
	var subdirs []entityPath
	var files []entityPath

	statusAndExistance := checkStatIsDirAndExists(path)
	if statusAndExistance != STATUS_EXISTS {
		return nil, nil, statusAndExistance
	}

	entries, _ := os.ReadDir(path)
	for _, entry := range entries {
		if entry.IsDir() {
			subdirs = append(subdirs, newEntityPath(entry.Name()))
		} else {
			files = append(files, newFilePath(entry.Name()))
		}
	}

	return subdirs, files, STATUS_IS_READ
}

func walkDir(path string) ([]walkedEntityPath, successStatus) {
	var results []walkedEntityPath

	statIsDir := checkStatIsDirAndExists(path)
	if statIsDir != STATUS_EXISTS {
		return nil, statIsDir
	}

	err := filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			results = append(results, newWalkedEntityPath(GLOBAL_DIRPATH, f.Name(), f.Size()))
		} else {
			results = append(results, newWalkedEntityPath(GLOBAL_FILEPATH, f.Name(), f.Size()))
		}

		return nil
	})

	if err != nil {
		return nil, STATUS_WALK_FAIL
	}

	return results, STATUS_WALK_SUCCESS
}

func checkStatIsDirAndExists(path string) successStatus {
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return STATUS_NOT_EXISTS
	}

	if !stat.IsDir() {
		return STATUS_ISNOTDIR
	}

	return STATUS_EXISTS
}

func hashString(str string) string {
	salt := strconv.Itoa(rand.Int())
	hasher := sha1.New()
	state, err := hasher.Write([]byte(salt))
	if state != 1 {
		errorOutStr("Error writing salt")
	}
	handleError(err)
	return hex.EncodeToString(hasher.Sum([]byte(str)))
}

func handleError(err error) {
	if err != nil {
		fmt.Printf("\033[1;31mError occured:\033[0m %s\n", err)
	}
}

func errorOutStr(err string) {
	fmt.Printf("\033[1;31mError occured:\033[0m %s\n", err)
	os.Exit(1)
}

// PTDP <version> <COMM> <hash/path>
// COMM:
//
//	CD_INIT
//	CD_SUBDIR
//	READ_FILE
//	STAT_ENTITY
//	LIST_DIR
//	LIST_FILES
//	LIST_SUBDIR
//	WALK_TREE
//
// version:
//
//	v1
func parseRequest(buffer []byte) (requestCode, string, bool) {
	parser := newRequestParser()
	switchCase := 0

	for _, b := range buffer {
		switch b {
		case 32:
			switchCase++
		default:
			parser.addNewByte(b, switchCase)
		}
	}

	pName := parser.protocolName.toStr()
	pVer := parser.protocolVersion.toStr()
	command := parser.command.toStr()
	pathOrHash := parser.protocolName.toStr()

	if pName != GLOBAL_PROTOCOL_NAME {
		return PARSE_ERROR_PNAME, "", false
	}

	if pVer != GLOBAL_VERSION_CONTROL {
		return PARSE_ERROR_PVER, "", false
	}

	if len(pathOrHash) < 2 {
		return PARSE_ERROR_PATH, "", false
	}

	if command == COMM_CD_INIT {
		return ACT_CD_INIT, pathOrHash, true
	} else if command == COMM_CD_SD {
		return ACT_CD_SUBDIR, pathOrHash, true
	} else if command == COMM_READ_BYTES {
		return ACT_READ_BYTES, pathOrHash, true
	} else if command == COMM_STAT {
		return ACT_STAT_ENTITY, pathOrHash, true
	} else if command == COMM_LIST_DIR {
		return ACT_LIST_DIR, pathOrHash, true
	} else if command == COMM_LIST_FILES {
		return ACT_LIST_FILES, pathOrHash, true
	} else if command == COMM_LIST_SUBDIRS {
		return ACT_LIST_SUBDIRS, pathOrHash, true
	} else if command == COMM_WAL_TREE {
		return ACT_WALK_TREE, pathOrHash, true
	} else {
		return PARSE_ERROR_COMM, pathOrHash, true
	}
}

func walkPathEntityCollectiveToString(paths []walkedEntityPath) string {
	finStr := ""
	for _, wep := range paths {
		finStr += "\n" + wep.toString()
	}

	return finStr
}

func parsePathOrHashTuple(pOrH string) (string, string) {
	first := ""
	second := ""
	index := 0

	for _, rune := range pOrH {
		switch rune {
		case GLOBAL_TUPLE_SEP:
			if index == 0 {
				index = 1
			}
		default:
			if index == 1 {
				first += string(rune)
			} else {
				second += string(rune)
			}
		}
	}

	return first, second
}

func (r responseCode) toString() string {
	respText := ""

	switch r {
	case RESPONSE_CD_INIT_OK:
		respText = "INIT_OK"
	case RESPONSE_CD_SUBDIR_OK:
		respText = "CD_OK"
	case RESPONSE_DIR_LISTED:
		respText = "DIR_LISTED"
	case RESPONSE_DIR_WALKED:
		respText = "DIR_WALKED"
	case RESPONSE_FILES_LISTED:
		respText = "FILES_LISTED"
	case RESPONSE_SUBDIRS_LISTED:
		respText = "SUBDIRS_LISTED"
	case RESPONSE_NO_DIR:
		respText = "NO_DIR"
	case RESPONSE_NO_EXIST:
		respText = "NO_EXISTS"
	case RESPONSE_NO_HASH:
		respText = "NO_HASH"
	case RESPONSE_PARSE_FAILED:
		respText = "PARSE_FAILED"
	case RESPONSE_NO_STATE:
		respText = "NO_STATE"
	case RESPONSE_READ_FAILED:
		respText = "READ_FAILED"
	case RESPONSE_STAT_ENTITY_OK:
		respText = "STAT_OK"
	case RESPONSE_STAT_FAILED:
		respText = "STAT_FAILED"
	case RESPONSE_WALK_FAILED:
		respText = "WALK_FAILED"
	}

	return fmt.Sprintf("%d - %s\n\n", r, respText)
}

func CleanUpProtoDir() {
	fmt.Printf("\nProtoGen's ProtoQuote server on TCP has been terminated\nRemoving socket file %s", socketPath)
	os.Remove(socketPath)
	os.Exit(0)
}