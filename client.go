/**
* Created by Visual Studio Code.
* User: tuxer
* Created At: 2018-01-15 15:24:41
**/

package jabber

import (
	"errors"
	"strings"
	"time"

	"gitlab.com/tuxer/go-logger"
	"gitlab.com/tuxer/go-xmpp"
)

//ClientListener ...
type ClientListener interface {
	ReceiveMessage(client *Client, from, session, body string)
	Connected()
	Disconnected()
}

//Client ...
type Client struct {
	listener ClientListener

	Username      string
	Password      string
	Server        string
	UseTLS        bool
	StartTLS      bool
	AllowInsecure bool

	lastSent     *time.Time
	lastReceived *time.Time
	running      bool

	xmppClient *xmpp.Client
	readCh     chan xmpp.Chat
}

//Start ...
func (c *Client) Start() error {
	useTLS := c.UseTLS
	startTLS := c.StartTLS
	allowInsecure := c.AllowInsecure

	log.D(`[Jabber]`, c.Username, `Connecting to`, c.Server)
	for {
		e := c.connect(useTLS, startTLS, allowInsecure)
		if e != nil {
			if useTLS {
				useTLS = false
			} else if !allowInsecure {
				allowInsecure = true
			} else if startTLS {
				startTLS = false
			} else {
				log.W(`[Jabber] Failed Connection`, c.Username, `Host:`, c.Server)
				log.D(e)
				return e
			}
		} else {
			break
		}
	}

	if c.listener != nil {
		c.listener.Connected()
	}
	log.I(`[Jabber]`, c.Username, `Connected.`)
	c.running = true
	go c.run()
	return nil
}

//GetLastReceived ...
func (c *Client) GetLastReceived() *time.Time {
	return c.lastReceived
}

//GetLastSent ...
func (c *Client) GetLastSent() *time.Time {
	return c.lastSent
}

//Stop ...
func (c *Client) Stop() {
	defer func() {
		if r := recover(); r != nil {
			log.D(r)
		}
	}()
	log.D(`[Jabber] Stopping`, c.Username, c.Server)
	c.running = false
	if c.listener != nil {
		c.listener.Disconnected()
	}
	c.xmppClient.Close()
}

//SetListener ...
func (c *Client) SetListener(listener ClientListener) {
	c.listener = listener
}

func (c *Client) connect(useTLS, startTLS, allowInsecure bool) error {
	var xmppClient *xmpp.Client
	options := xmpp.Options{
		Host:                         c.Server,
		User:                         c.Username,
		Password:                     c.Password,
		NoTLS:                        !useTLS,
		Debug:                        false,
		Session:                      true,
		Status:                       "xa",
		StatusMessage:                "",
		StartTLS:                     startTLS,
		InsecureAllowUnencryptedAuth: allowInsecure,
	}

	xmppClient, e := options.NewClient()

	if e != nil {
		return e
	}
	c.xmppClient = xmppClient

	return nil
}

func (c *Client) run() {
	for c.running {
		packet, e := c.xmppClient.Recv()
		if e != nil {
			log.E(e)
			c.Stop()

			go func() {
				log.D(`[Jabber] Retry connection`)
				time.Sleep(5 * time.Second)
				c.Start()
			}()
			return
		}
		switch chat := packet.(type) {
		case xmpp.Chat:
			from := strings.Split(chat.Remote, `/`)
			log.D(`[Jabber] Receive from`, from)
			if chat.Text != `` {
				lastReceived := time.Now()
				c.lastReceived = &lastReceived
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.E(r)
						}
					}()
					if c.listener != nil {
						c.listener.ReceiveMessage(c, from[0], from[1], chat.Text)
					} else {
						log.D(`[Jabber] From`, chat.Remote, ":", chat.Text)
					}
					if c.readCh != nil {
						c.readCh <- chat
						close(c.readCh)
					}
				}()
			}
		case xmpp.Presence:
		}
	}
	c.running = false
}

//SendMessage ...
func (c *Client) SendMessage(to, body string) error {
	log.D(`[Jabber] To`, to, ":", body)
	now := time.Now()
	c.lastSent = &now

	if c.xmppClient == nil {
		return errors.New(`Supplier not available`)
	}
	_, e := c.xmppClient.Send(xmpp.Chat{Remote: to, Type: "chat", Text: body})
	return e
}

func (c *Client) IsRunning() bool {
	return c.running
}

func NewClient(server, username, password string) *Client {
	return &Client{Server: server, Username: username, Password: password, UseTLS: true, StartTLS: true, AllowInsecure: false}
}
