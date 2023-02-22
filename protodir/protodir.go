package protodir

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type pathType int
type requestCode int
type responseCode int
type successStatus int
type bArr []byte

const (
	GLOBAL_VERSION_CONTROL     string        = "v1"
	GLOBAL_PROTOCOL_NAME       string        = "PTDP"
	GLOBAL_DESTROY_READER      string        = "()"
	GLOBAL_HEADER_PREFIX       string        = "$"
	GLOBAL_LIST_SUBDIRS_HEADER string        = "LIST_SUBDIRS"
	GLOBAL_LIST_FILES_HEADER   string        = "LIST_FILES"
	GLOBAL_LIST_DIR_HEADER     string        = "LIST_DIR"
	GLOBAL_STAT_HEADER         string        = "STAT_ENTITY"
	GLOBAL_WALK_HEADER         string        = "WALK_TREE"
	GLOBAL_READ_HEADER         string        = "READ_BYTES"
	GLOBAL_TRIMMER             string        = " \n\r\x00"
	GLOBAL_TUPLE_SEP           string        = ";"
	COMM_INIT_STATE            string        = "INIT_STATE"
	COMM_CD_SD                 string        = "CD_SUBDIR"
	COMM_STAT                  string        = "STAT_ENTITY"
	COMM_READ_BYTES            string        = "READ_BYTES"
	COMM_LIST_DIR              string        = "LIST_DIR"
	COMM_LIST_FILES            string        = "LIST_FILES"
	COMM_LIST_SUBDIRS          string        = "LIST_SUBDIRS"
	COMM_LIST_STATES           string        = "LIST_STATES"
	COMM_WAL_TREE              string        = "WALK_TREE"
	ERR_PARSE_COMM             string        = "ERROR_PARSE_COMM"
	ERR_WRONG_COMM             string        = "ERROR_WRONG_COMM"
	ERR_PARSE_HASH             string        = "ERROR_PARSE_PATH_OR_HASH"
	ERR_PARSE_PNAME            string        = "ERROR_PARSE_PROTOCOL_NAME"
	ERR_PARSE_PVER             string        = "ERROR_PARSE_VERSION_CONTROL"
	ERR_TWO_HASH               string        = "ERROR_NEEDS_TWO_HASH"
	GLOBAL_FILEPATH            pathType      = 31
	GLOBAL_DIRPATH             pathType      = 32
	ACT_INIT_STATE             requestCode   = 0
	ACT_CD_SUBDIR              requestCode   = 12
	ACT_LIST_DIR               requestCode   = 22
	ACT_READ_BYTES             requestCode   = 32
	ACT_STAT_ENTITY            requestCode   = 42
	ACT_LIST_SUBDIRS           requestCode   = 52
	ACT_LIST_FILES             requestCode   = 62
	ACT_WALK_TREE              requestCode   = 72
	ACT_LIST_STATE             requestCode   = 82
	PARSE_ERROR_PNAME          requestCode   = 10
	PARSE_ERROR_PVER           requestCode   = 11
	PARSE_ERROR_COMM           requestCode   = 13
	PARSE_ERROR_PATH           requestCode   = 14
	RESPONSE_NEED_TWO_HASH     responseCode  = 15
	RESPONSE_INIT_STATE_OK     responseCode  = 12
	RESPONSE_CD_SUBDIR_OK      responseCode  = 13
	RESPONSE_READ_FILE_OK      responseCode  = 22
	RESPONSE_STAT_ENTITY_OK    responseCode  = 23
	RESPONSE_DIR_LISTED        responseCode  = 32
	RESPONSE_FILES_LISTED      responseCode  = 33
	RESPONSE_SUBDIRS_LISTED    responseCode  = 34
	RESPONSE_DIR_WALKED        responseCode  = 42
	RESPONSE_LISTED_STATES     responseCode  = 52
	RESPONSE_PARSE_FAILED      responseCode  = 100
	RESPONSE_NO_DIR            responseCode  = 110
	RESPONSE_NO_HASH           responseCode  = 120
	RESPONSE_NO_STATE          responseCode  = 130
	RESPONSE_NO_EXIST          responseCode  = 140
	RESPONSE_WALK_FAILED       responseCode  = 150
	RESPONSE_READ_FAILED       responseCode  = 160
	RESPONSE_STAT_FAILED       responseCode  = 170
	RESPONSE_WRONG_COMM        responseCode  = 180
	RESPONSE_IS_NOT_FILE       responseCode  = 190
	RESPONSE_IS_NOT_DIR        responseCode  = 200
	STATUS_NOT_EXISTS          successStatus = 0
	STATUS_NO_HASH             successStatus = 1
	STATUS_EXISTS              successStatus = 3
	STATUS_ISNOTDIR            successStatus = 4
	STATUS_ISNOTFILE           successStatus = 5
	STATUS_IS_READ             successStatus = 6
	STATUS_DID_CD              successStatus = 7
	STATUS_WALK_FAIL           successStatus = 8
	STATUS_WALK_SUCCESS        successStatus = 9
	STATUS_DID_STAT            successStatus = 10
	STATUS_DID_FAIL            successStatus = 11
	STATUS_DID_SPLIT           successStatus = 12
	STATUS_SPLIT_FAIL          successStatus = 13
)

var (
	globalTtl            = 10
	globalCleareInterval = 30
	socketPath           = ""
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

func ProtoDirMain(sockPath string, globalTtlSet, globalCleareIntervalSet int) {
	globalCleareInterval = globalCleareIntervalSet
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

func newPathCollective(root string) pathCollective {
	return pathCollective{
		rootDir: root,
		currDir: "UNSET",
		subdirs: make([]entityPath, 0),
		files:   make([]entityPath, 0),
	}
}

func newPathState(root string) (pathState, string) {
	pState := pathState{
		path: newPathCollective(root),
		hash: hashString(root),
	}
	go pState.waitForGC()

	return pState, pState.hash
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

	var bResp []byte
	var code responseCode

	if !success {
		bResp, code = pdr.handleRequestFailure(req)
	} else if req == ACT_CD_SUBDIR || req == ACT_READ_BYTES || req == ACT_STAT_ENTITY {
		bResp, code = pdr.handleDoubleHashRequest(req, pathOrHash)
	} else {
		bResp, code = pdr.handleSingleHashRequest(req, pathOrHash)
	}

	bResp = append(bResp, 10)
	bResp = append(bResp, 10)

	return bResp, code
}

func (*protoDirState) handleRequestFailure(req requestCode) ([]byte, responseCode) {
	var bResp []byte
	var code responseCode

	if req == PARSE_ERROR_COMM {
		bResp, code = []byte(ERR_PARSE_COMM), RESPONSE_PARSE_FAILED
	} else if req == PARSE_ERROR_PATH {
		bResp, code = []byte(ERR_PARSE_HASH), RESPONSE_PARSE_FAILED
	} else if req == PARSE_ERROR_PNAME {
		bResp, code = []byte(ERR_PARSE_PNAME), RESPONSE_PARSE_FAILED
	} else if req == PARSE_ERROR_PVER {
		bResp, code = []byte(ERR_PARSE_PVER), RESPONSE_PARSE_FAILED
	}

	return bResp, code
}

func (pdr *protoDirState) handleDoubleHashRequest(req requestCode, doubleHash string) ([]byte, responseCode) {
	stateHash, entityHash, success := parsePathOrHashTuple(doubleHash)
	if success != STATUS_DID_SPLIT {
		return []byte(ERR_TWO_HASH), RESPONSE_PARSE_FAILED
	}

	var bResp []byte
	var code responseCode

	if req == ACT_CD_SUBDIR {
		code = pdr.handleRequestCDSubDir(stateHash, entityHash)
		bResp = []byte{}
	} else if req == ACT_READ_BYTES {
		bResp, code = pdr.handleRequestReadFile(stateHash, entityHash)
	} else if req == ACT_STAT_ENTITY {
		bResp, code = pdr.handleRequestStat(stateHash, entityHash)
	}

	return bResp, code
}

func (pdr *protoDirState) handleSingleHashRequest(req requestCode, pathOrHash string) ([]byte, responseCode) {
	var bResp []byte
	var code responseCode

	if req == ACT_INIT_STATE {
		bResp, code = pdr.handleRequestInit(pathOrHash)
	} else if req == ACT_LIST_DIR {
		bResp, code = pdr.handleRequestWholeDir(pathOrHash)
	} else if req == ACT_LIST_SUBDIRS {
		bResp, code = pdr.handleRequestListSubDirs(pathOrHash)
	} else if req == ACT_LIST_FILES {
		bResp, code = pdr.handleRequestListFiles(pathOrHash)
	} else if req == ACT_WALK_TREE {
		bResp, code = pdr.handleRequestWalkDir(pathOrHash)
	} else if req == ACT_LIST_STATE {
		bResp, code = pdr.handleRequestListStates()
	} else {
		bResp, code = []byte(ERR_WRONG_COMM), RESPONSE_WRONG_COMM
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

func (pdr *protoDirState) addNewState(rootDir string) string {
	defer pdr.Unlock()
	pdr.Lock()

	newState, hashState := newPathState(rootDir)
	pdr.states = append(pdr.states, newState)

	return hashState
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
	for i, state := range pdr.states {
		if state.matchHash(hash) {
			return &pdr.states[i]
		}
	}

	return nil
}

func (pdr *protoDirState) loopAndWaitForClearNUll() {
	for {
		time.Sleep(time.Hour * time.Duration(globalCleareInterval))
		pdr.cleanNull()
	}
}

func (pdr *protoDirState) handleRequestInit(path string) ([]byte, responseCode) {
	hashState := pdr.addNewState(path)

	return []byte(trimHash(hashState)), RESPONSE_INIT_STATE_OK
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

func (pdr *protoDirState) handleRequestListStates() ([]byte, responseCode) {
	listStates := GLOBAL_HEADER_PREFIX + "LIST_STATES;\n"
	for _, state := range pdr.states {
		listStates += "\n"
		listStates += state.toString()
	}

	if len(listStates) == 0 {
		listStates = "NO_STATES"
	}

	listStates += "\n\n"

	return []byte(listStates), RESPONSE_LISTED_STATES
}

func (pdr *protoDirState) handleRequestListSubDirs(hashState string) ([]byte, responseCode) {
	state := pdr.filterStatesAndReturn(hashState)
	if state == nil {
		return nil, RESPONSE_NO_STATE
	}

	res := state.path.dirsToString()

	return addHeader(state.path.currDir, GLOBAL_LIST_SUBDIRS_HEADER, []byte(res)), RESPONSE_SUBDIRS_LISTED
}

func (pdr *protoDirState) handleRequestListFiles(hashState string) ([]byte, responseCode) {
	state := pdr.filterStatesAndReturn(hashState)
	if state == nil {
		return nil, RESPONSE_NO_STATE
	}

	res := state.path.filesToSting()

	return addHeader(state.path.currDir, GLOBAL_LIST_FILES_HEADER, []byte(res)), RESPONSE_FILES_LISTED
}

func (pdr *protoDirState) handleRequestWholeDir(hashState string) ([]byte, responseCode) {
	state := pdr.filterStatesAndReturn(hashState)
	if state == nil {
		return nil, RESPONSE_NO_STATE
	}

	res := state.path.dirsAndFilesToString()

	return addHeader(state.path.currDir, GLOBAL_LIST_DIR_HEADER, []byte(res)), RESPONSE_DIR_LISTED
}

func (pdr *protoDirState) handleRequestWalkDir(hashState string) ([]byte, responseCode) {
	state := pdr.filterStatesAndReturn(hashState)
	if state == nil {
		return nil, RESPONSE_NO_STATE
	}

	walked, stat := state.path.walkDirAndToBytes()
	if stat == STATUS_ISNOTDIR {
		return nil, RESPONSE_IS_NOT_DIR
	} else if stat == STATUS_NOT_EXISTS {
		return nil, RESPONSE_NO_EXIST
	} else if stat == STATUS_WALK_FAIL {
		return nil, RESPONSE_WALK_FAILED
	}

	return addHeader(state.path.currDir, GLOBAL_WALK_HEADER, walked), RESPONSE_DIR_WALKED
}

func (pdr *protoDirState) handleRequestReadFile(hashState, hashFile string) ([]byte, responseCode) {
	state := pdr.filterStatesAndReturn(hashState)
	if state == nil {
		return nil, RESPONSE_NO_STATE
	}

	read, path, stat := state.path.filterAndReadFile(hashFile)

	if stat == STATUS_ISNOTFILE {
		return nil, RESPONSE_IS_NOT_FILE
	} else if stat == STATUS_NOT_EXISTS {
		return nil, RESPONSE_NO_EXIST
	} else if stat == STATUS_NO_HASH {
		return nil, RESPONSE_NO_HASH
	}

	return addHeader(path, GLOBAL_READ_HEADER, read), RESPONSE_READ_FILE_OK
}

func (pdr *protoDirState) handleRequestStat(hashState, hashEntity string) ([]byte, responseCode) {
	state := pdr.filterStatesAndReturn(hashState)
	if state == nil {
		return nil, RESPONSE_NO_STATE
	}

	entityStat, path, stat := state.path.filterAndStatEntity(hashEntity)

	if stat == STATUS_NOT_EXISTS {
		return nil, RESPONSE_NO_EXIST
	} else if stat == STATUS_NO_HASH {
		return nil, RESPONSE_NO_HASH
	}

	return addHeader(path, GLOBAL_STAT_HEADER, entityStat), RESPONSE_STAT_ENTITY_OK
}

func (ep entityPath) readFile(rootDir string) ([]byte, string, successStatus) {
	if ep.ty == GLOBAL_DIRPATH {
		return nil, "", STATUS_ISNOTFILE
	}

	path := filepath.Join(rootDir, ep.path)
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, "", STATUS_NOT_EXISTS
	}

	return contents, path, STATUS_IS_READ
}

func (ep entityPath) statEntity(rootDir string) ([]byte, string, successStatus) {
	path := filepath.Join(rootDir, ep.path)
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, "", STATUS_NOT_EXISTS
	}

	statString := fmt.Sprintf(`IsDir: %t;
ModTime: %s;
Mode: %s;
Name: %s;
Size: %d;
	`, stat.IsDir(), stat.ModTime(), stat.Mode().String(), stat.Name(), stat.Size())

	return []byte(statString), path, STATUS_DID_STAT
}

func (ep entityPath) matchHash(hash string) bool {
	return trimHash(ep.hash) == hash
}

func (ep entityPath) getHashTrimmed() string {
	return trimHash(ep.hash)
}

func (ep entityPath) toString() string {
	if ep.ty == GLOBAL_DIRPATH {
		return fmt.Sprintf("+d+path=%s+hash=%s", ep.path, ep.getHashTrimmed())
	} else {
		return fmt.Sprintf("*f*path=%s*hash=%s", ep.path, ep.getHashTrimmed())
	}
}

func (wep walkedEntityPath) toString() string {
	if wep.ty == GLOBAL_DIRPATH {
		return fmt.Sprintf("+d+path=%s+size=%d", wep.path, wep.size)
	} else {
		return fmt.Sprintf("*f*path=%s*size=%d", wep.path, wep.size)
	}
}

func (ep pathCollective) filesToSting() string {
	finStr := ""

	for _, entity := range ep.files {
		finStr += "\n" + entity.toString()
	}

	if len(finStr) == 0 {
		finStr = "NO_FILE"
	}

	return finStr
}

func (ep pathCollective) dirsToString() string {
	finStr := ""

	for _, entity := range ep.subdirs {
		finStr += "\n" + entity.toString()
	}

	if len(finStr) == 0 {
		finStr = "NO_DIR"
	}

	return finStr
}

func (ep pathCollective) dirsAndFilesToString() string {
	fileStr := ""
	dirsStr := ""

	for _, entity := range ep.files {
		fileStr += "\n" + entity.toString()
	}

	for _, entity := range ep.subdirs {
		dirsStr += "\n" + entity.toString()
	}

	if len(fileStr) == 0 {
		fileStr = "NO_FILE"
	}

	if len(dirsStr) == 0 {
		dirsStr = "NO_DIR"
	}

	return fmt.Sprintf("%s\n===\n%s\n", fileStr, dirsStr)
}

func (p *pathCollective) getSubDirByHash(hash string) *entityPath {
	for i, sd := range p.subdirs {
		if sd.matchHash(hash) {
			return &p.subdirs[i]
		}
	}

	return nil
}

func (p *pathCollective) getFileByHash(hash string) *entityPath {
	for i, f := range p.files {
		if f.matchHash(hash) {
			return &p.files[i]
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

	return STATUS_DID_CD
}

func (p *pathCollective) setFilesAndSubDirs() successStatus {
	newSubDirs, newFiles, result := getFilesAndSubdirsInAFolder(p.currDir)

	if result != STATUS_IS_READ {
		return result
	}

	p.subdirs, p.files = newSubDirs, newFiles
	return result
}

func (p *pathCollective) filterAndReadFile(hash string) ([]byte, string, successStatus) {
	filePath := p.getFileByHash(hash)
	if filePath == nil {
		return nil, "", STATUS_NO_HASH
	}

	return filePath.readFile(p.currDir)
}

func (p pathCollective) walkDirAndToBytes() ([]byte, successStatus) {
	walked, stat := walkDir(p.currDir)
	if stat != STATUS_WALK_SUCCESS {
		return nil, stat
	}

	str := walkPathEntityCollectiveToString(walked)

	return []byte(str), STATUS_WALK_SUCCESS
}

func (p pathCollective) filterAndStatEntity(hash string) ([]byte, string, successStatus) {
	entity := p.getFileByHash(hash)
	if entity == nil {
		entity = p.getSubDirByHash(hash)
	}
	if entity == nil {
		return nil, "", STATUS_NO_HASH
	}

	fileOrDirState, path, stat := entity.statEntity(p.currDir)

	if stat != STATUS_DID_STAT {
		return nil, "", stat
	}

	return fileOrDirState, path, stat
}

func (ps *pathState) cdAndSetFilesAndSubdirs(hash string) successStatus {
	var stat successStatus

	if ps.matchHash(hash) {
		stat = ps.path.cdToRoot()
	} else {
		stat = ps.path.cdToSubDir(hash)
	}

	if stat != STATUS_DID_CD {
		return stat
	}

	stat = ps.path.setFilesAndSubDirs()

	return stat
}

func (ps *pathState) setForGC() {
	ps.hash = GLOBAL_DESTROY_READER
}

func (ps *pathState) waitForGC() {
	time.Sleep(time.Minute * time.Duration(globalTtl))
	ps.setForGC()
}
func (ps *pathState) matchHash(hash string) bool {
	return trimHash(ps.hash) == hash
}

func (ps *pathState) getHashTrimmed() string {
	return trimHash(ps.hash)
}

func (ps *pathState) toString() string {
	return fmt.Sprintf("^s^cd=%s^hash=%s", ps.path.currDir, ps.getHashTrimmed())
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
	hasher := sha512.New()
	hasher.Write([]byte(salt))
	hash := hex.EncodeToString(hasher.Sum([]byte(str)))
	return hash
}

func handleError(err error) {
	if err != nil {
		fmt.Printf("\033[1;31mError occured:\033[0m %s\n", err)
	}
}

// PTDP <version> <COMM> <hash/path>
// COMM:
//
//	INIT_STATE
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
	var prevByte byte

	for _, b := range buffer {
		switch b {
		case 32:
			if prevByte != 32 {
				switchCase++
			}
		default:
			parser.addNewByte(b, switchCase)
		}

		prevByte = b
	}

	pName := parser.protocolName.toStr()
	pVer := parser.protocolVersion.toStr()
	command := parser.command.toStr()
	pathOrHash := parser.pathOrHash.toStr()

	pathOrHash = strings.Trim(pathOrHash, GLOBAL_TRIMMER)

	if pName != GLOBAL_PROTOCOL_NAME {
		return PARSE_ERROR_PNAME, "", false
	}

	if pVer != GLOBAL_VERSION_CONTROL {
		return PARSE_ERROR_PVER, "", false
	}

	if !strings.Contains(command, COMM_LIST_STATES) {
		if len(pathOrHash) < 2 {
			return PARSE_ERROR_PATH, "", false
		}
	}

	if strings.Contains(command, COMM_INIT_STATE) {
		return ACT_INIT_STATE, pathOrHash, true
	} else if strings.Contains(command, COMM_CD_SD) {
		return ACT_CD_SUBDIR, pathOrHash, true
	} else if strings.Contains(command, COMM_READ_BYTES) {
		return ACT_READ_BYTES, pathOrHash, true
	} else if strings.Contains(command, COMM_STAT) {
		return ACT_STAT_ENTITY, pathOrHash, true
	} else if strings.Contains(command, COMM_LIST_DIR) {
		return ACT_LIST_DIR, pathOrHash, true
	} else if strings.Contains(command, COMM_LIST_FILES) {
		return ACT_LIST_FILES, pathOrHash, true
	} else if strings.Contains(command, COMM_LIST_SUBDIRS) {
		return ACT_LIST_SUBDIRS, pathOrHash, true
	} else if strings.Contains(command, COMM_WAL_TREE) {
		return ACT_WALK_TREE, pathOrHash, true
	} else if strings.Contains(command, COMM_LIST_STATES) {
		return ACT_LIST_STATE, pathOrHash, true
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

func parsePathOrHashTuple(pOrH string) (string, string, successStatus) {
	trimmed := strings.Trim(pOrH, GLOBAL_TRIMMER)
	split := strings.Split(trimmed, GLOBAL_TUPLE_SEP)

	if len(split) != 2 {
		return "", "", STATUS_SPLIT_FAIL
	}

	return split[0], split[1], STATUS_DID_SPLIT
}

func (r responseCode) toString() string {
	respText := ""

	switch r {
	case RESPONSE_INIT_STATE_OK:
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
	case RESPONSE_WRONG_COMM:
		respText = "WRONG_COMMAND"
	case RESPONSE_LISTED_STATES:
		respText = "STATES_LISTED"
	case RESPONSE_READ_FILE_OK:
		respText = "BYTES_READ"
	case RESPONSE_IS_NOT_DIR:
		respText = "IS_NOT_DIR"
	}

	return fmt.Sprintf("%d - %s\n\n", r, respText)
}

func CleanUpProtoDir() {
	fmt.Printf("\nProtoGen's ProtoQuote server on TCP has been terminated\nRemoving socket file %s\n", socketPath)
	os.Remove(socketPath)
	os.Exit(0)
}

func trimHash(hash string) string {
	fin := ""
	for i, rune := range hash {
		if i%20 == 0 {
			fin += string(rune)
		}
	}

	return fin
}

func addHeader(path, headerSet string, contents []byte) []byte {
	header := []byte(fmt.Sprintf("%s%s: %s;\n", GLOBAL_HEADER_PREFIX, headerSet, path))
	contentsWithHeader := header
	contentsWithHeader = append(contentsWithHeader, contents...)

	return contentsWithHeader
}
