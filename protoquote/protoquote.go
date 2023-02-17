package protoquote

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
)

type quoteResponse struct {
	Id         string   `json:"_id"`
	Conent     string   `json:"content"`
	Author     string   `json:"author"`
	AuthorSlug string   `json:"authorSlug"`
	Length     int      `json:"length"`
	Tags       []string `json:"tags"`
}

func (qresp *quoteResponse) filterAuthorAndQute() (string, string) {
	return qresp.Author, qresp.Conent
}

func ProtoQuoteMain(addr string) {
	tcpListener, err := net.Listen("tcp", addr)
	handleError(err)

	handleTcpListener(tcpListener)
}

func handleTcpListener(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		handleError(err)

		handleConn(conn)
	}
}

func parsequoteResponse(inResp []byte) quoteResponse {
	var qresponse quoteResponse
	json.Unmarshal(inResp, &qresponse)

	return qresponse
}

func getAuthorAndQuote() (string, string) {
	resp, err := http.Get("https://api.quotable.io/random")
	handleError(err)

	if resp.StatusCode == 429 {
		return "Jay Jonah Jameson", "Sorry, Peter, this damn API has a limit of 180 quotes per minute. You ran out of luck! Now go smoke that Marijuana out, if you catch my drift!"
	}

	body, err := io.ReadAll(resp.Body)
	handleError(err)

	qresp := parsequoteResponse(body)

	return qresp.filterAuthorAndQute()
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	author, quote := getAuthorAndQuote()
	applicationFrame := fmt.Sprintf("\033[1;33mSup my friend from Reddit!\033[0m Remember not to accept candies from strangers, especially ones wearing fursuits!\n%s says: \"%s\"\nThis was a message sent to you via a Transfer Control Protocl (TCP). \n\n -Chubak, github.com/chubek", author, quote)

	conn.Write([]byte(applicationFrame))
}

func handleError(err error) {
	fmt.Printf("\033[1;31mError occured:\033[0m %s\n", err)
}
