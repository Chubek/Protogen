package protoprime

type dummy struct {}
type semaphoreChan chan dummy
type Semaphore struct {
	chans 			semaphoreChan
	numResources 	int
	currHoldNum 	int
}

func NewSemaphore(numResouces int) Semaphore {
	return Semaphore {  
		chans: make(semaphoreChan , numResouces), 
		numResources: numResouces, 
		currHoldNum: 0,
	}
}

func (sem *Semaphore) blockExec(holdNum int) {
	if sem.currHoldNum > 0 {
		sem.currHoldNum += holdNum 
	} else {
		sem.currHoldNum = holdNum
	}
	
	for i := 0; i <= holdNum; i++ {
		sem.chans <- dummy {}
	}
}

func (sem *Semaphore) releaseExec() {
	for i := 0; i <= sem.currHoldNum; i++ {
		<- sem.chans 
	}
}

func (sem *Semaphore) PrimMutexLock() {
	sem.blockExec(1)
} 

func (sem *Semaphore) PrimMutexUnLock() {
	sem.releaseExec()
}

func (sem *Semaphore) PrimWait(numHold int) {
	sem.blockExec(numHold)
}

func (sem *Semaphore) PrimSignal() {
	sem.releaseExec()
}