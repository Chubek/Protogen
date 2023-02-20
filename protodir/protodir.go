package protodir

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
	"net"
)

type pathType int
type requestCode int
type responseCode int
type entityStat int
type bArr []byte

const (
	GLOBAL_VERSION_CONTROL  string       = "v1"
	GLOBAL_PROTOCOL_NAME    string       = "PTDP"
	GLOBAL_DESTROY_READER   string       = "()"
	GLOBAL_TTL              int          = 10
	GLOBAL_CLEAR_INTERVAL   int          = 1
	GLOBAL_FILEPATH         pathType     = 31
	GLOBAL_DIRPATH          pathType     = 32
	COMM_CD_INIT            string       = "CD_INIT"
	COMM_CD_SD              string       = "CD_SUBDIR"
	COMM_STAT               string       = "STAT"
	COMM_READ_BYTES         string       = "READ_BYTES"
	COMM_LIST_DIR           string       = "LIST_DIR"
	COMM_LIST_FILES         string       = "LIST_FILES"
	COMM_LIST_SUBDIRS       string       = "LIST_SUBDIRS"
	COMM_WAL_TREE           string       = "WALK_TREE"
	ACT_CD_INIT             requestCode  = 0
	ACT_CD_SUBDIR           requestCode  = 1
	ACT_LIST_DIR            requestCode  = 2
	ACT_READ_BYTES          requestCode  = 3
	ACT_STAT_ENTITY         requestCode  = 4
	ACT_LIST_SUBDIRS        requestCode  = 5
	ACT_LIST_FILES          requestCode  = 6
	ACT_WALK_TREE           requestCode  = 9
	PARSE_ERROR_PNAME       requestCode  = 10
	PARSE_ERROR_PVER        requestCode  = 11
	PARSE_ERROR_COMM        requestCode  = 13
	PARSE_ERROR_PATH        requestCode  = 14
	RESPONSE_CD_INIT_OK     responseCode = 120
	RESPONSE_CD_SUBDIR_OK   responseCode = 130
	RESPONSE_READ_FILE_OK   responseCode = 140
	RESPONSE_STAT_ENTITY_OK responseCode = 150
	RESPONSE_PARSE_FAILED   responseCode = 160
	RESPONSE_NO_DIR         responseCode = 170
	RESPONSE_NO_HASH        responseCode = 180
	RESPONSE_NO_STATE		responseCode = 190
	RESPONSE_NO_EXIST		responseCode = 200
	RESPONSE_DIR_LISTED		responseCode = 300
	RESPONSE_FILES_LISTED	responseCode = 400
	RESPONSE_SUBDIRS_LISTED	responseCode = 500
	RESPONSE_DIR_WALKED		responseCode = 600
	STAT_NOT_EXISTS		entityStat = 0
	STAT_NO_HASH			entityStat	= 1
	STAT_EXISTS			entityStat = 3
	STAT_ISNOTDIR			entityStat = 4
	STAT_ISNOTFILE			entityStat = 5
	STAT_IS_READ			entityStat = 6
	STAT_DID_CD				entityStat = 7
)

type entityPath struct {
	ty   pathType
	path string
	hash string
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

func initProtoDirState() protoDirState {
	state := protoDirState{
		states: make([]pathState, 0),
	}
	go state.loopAndWaitForClearNUll()

	return state
}

func newFilePath(path string) entityPath {
	return entityPath{
		path: path,
		hash: hashString(path),
		ty:   GLOBAL_FILEPATH,
	}
}

func newDirPath(path string) entityPath {
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
	return pathState{
		path: newPath(root),
		hash: hashString(root),
	}
}

func newBArr() bArr {
	return make(bArr, 0)
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

	if stat == STAT_ISNOTDIR {
		return RESPONSE_NO_DIR
	} else if stat == STAT_NOT_EXISTS {
		return RESPONSE_NO_EXIST
	} else if stat == STAT_NO_HASH {
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

func (st *pathState) cdAndSetFilesAndSubdirs(hash string) entityStat {
	var stat entityStat

	if hash == st.hash {
		stat = st.path.cdToRoot()
	} else {
		stat = st.path.cdToSubDir(hash)
	}

	if stat != STAT_DID_CD {
		return stat
	}

	stat = st.path.setFilesAndSubDirs()

	return stat
}

func (ep entityPath) readFile() ([]byte, entityStat) {
	if ep.ty == GLOBAL_DIRPATH {
		return nil, STAT_ISNOTFILE
	}

	contents, err := os.ReadFile(ep.path)
	if os.IsNotExist(err) {
		return nil, STAT_NOT_EXISTS
	}

	return contents, STAT_EXISTS
}

func (ep entityPath) statEntity() ([]byte, bool) {
	stat, err := os.Stat(ep.path)
	if err != nil {
		return nil, false
	}

	statString := fmt.Sprintf(`
	IsDir: %t;
	ModTime: %s;
	Mode: %s;
	Name: %s;
	Size: %d;
	Sys: %s;
	`, stat.IsDir(), stat.ModTime(), stat.Mode().String(), stat.Name(), stat.Size(), stat.Sys())

	return []byte(statString), true
}

func (ep entityPath) toString() string {
	if ep.ty == GLOBAL_DIRPATH {
		return fmt.Sprintf("+d\t\t\t+p -> %s\t\t\t+sha-1 -> %s", ep.path, ep.hash)
	} else {
		return fmt.Sprintf("*f\t\t\t*p - > %s\t\t\t*sha-1 -> %s", ep.path, ep.hash)
	}
}

func (ep pathCollective) filesToSting() string {
	finStr := ""

	for _, entity := range ep.files {
		finStr = fmt.Sprintf("%s\n%s", finStr, entity.toString())
	}

	return finStr
}

func (ep pathCollective) dirsToString() string {
	finStr := ""

	for _, entity := range ep.subdirs {
		finStr = fmt.Sprintf("%s\n%s", finStr, entity.toString())
	}

	return finStr
}

func (ep pathCollective) dirsAndFilesToString() string {
	finStr := ""

	for _, entity := range ep.subdirs {
		finStr = fmt.Sprintf("%s\n%s", finStr, entity.toString())
	}

	finStr += "\n"

	for _, entity := range ep.files {
		finStr = fmt.Sprintf("%s\n%s", finStr, entity.toString())
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

func (p *pathCollective) cdToSubDir(subdirHash string) entityStat {
	subDir := p.getSubDirByHash(subdirHash)
	if subDir == nil {
		return STAT_NO_HASH
	}
	joinedPath := filepath.Join(p.rootDir, subDir.path)
	stat := checkStatIsDirAndExists(joinedPath)

	if stat != STAT_EXISTS {
		p.currDir = joinedPath
		return STAT_DID_CD
	}

	return stat
}

func (p *pathCollective) cdToRoot() entityStat {
	stat := checkStatIsDirAndExists(p.rootDir)
	if stat == STAT_EXISTS {
		p.currDir = p.rootDir
	}

	return stat
}

func (p *pathCollective) setFilesAndSubDirs() entityStat {
	newSubDirs, newFiles, result := getFilesAndSubdirsInAFolder(p.currDir)

	if result != STAT_IS_READ {
		return result
	}

	p.subdirs, p.files = newSubDirs, newFiles
	return result
}

func (p *pathCollective) filterAndReadFile(hash string) ([]byte, entityStat) {
	filePath := p.getFileByHash(hash)
	if filePath == nil {
		return nil, STAT_NO_HASH
	}

	return filePath.readFile()
}

func (ps *pathState) setForGC() {
	ps.hash = GLOBAL_DESTROY_READER
}

func (ps *pathState) waitForGC() {
	time.Sleep(time.Minute * time.Duration(GLOBAL_TTL))
	ps.setForGC()
}

func (ps *pathState) matchHash(hash string) bool {
	if ps.hash == hash {
		return true
	}

	return false
}

func getFilesAndSubdirsInAFolder(path string) ([]entityPath, []entityPath, entityStat) {
	var subdirs []entityPath
	var files []entityPath

	statusAndExistance := checkStatIsDirAndExists(path)
	if statusAndExistance != STAT_EXISTS {
		return nil, nil, statusAndExistance
	}

	entries, _ := os.ReadDir(path)
	for _, entry := range entries {
		if entry.IsDir() {
			subdirs = append(subdirs, newDirPath(entry.Name()))
		} else {
			files = append(files, newFilePath(entry.Name()))
		}
	}

	
	return subdirs, files, STAT_IS_READ
}

func checkStatIsDirAndExists(path string) entityStat {
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return STAT_NOT_EXISTS
	}

	if !stat.IsDir() {
		return STAT_ISNOTDIR
	}

	return STAT_EXISTS
}

func checkStatIsFileAndExists(path string) entityStat {
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return STAT_NOT_EXISTS
	}

	if stat.IsDir() {
		return STAT_ISNOTFILE
	}

	return STAT_EXISTS
}


func hashString(str string) string {
	salt := string(rand.Int())
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
