package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/go-homedir"
	"github.com/textileio/textile-go/core"
	"github.com/textileio/textile-go/repo"
	"github.com/textileio/textile-go/schema"
	"github.com/textileio/textile-go/schema/textile"
	"gopkg.in/abiosoft/ishell.v2"
)

var errMissingThreadId = errors.New("missing thread id")

func init() {
	register(&threadsCmd{})
}

type threadsCmd struct {
	Add        addThreadsCmd        `command:"add"`
	List       lsThreadsCmd         `command:"ls"`
	Get        getThreadsCmd        `command:"get"`
	GetDefault getDefaultThreadsCmd `command:"default"`
	Remove     rmThreadsCmd         `command:"rm"`
	Stream     streamThreadsCmd     `command:"updates"`
}

func (x *threadsCmd) Name() string {
	return "threads"
}

func (x *threadsCmd) Short() string {
	return "Manage threads"
}

func (x *threadsCmd) Long() string {
	return `
Threads are distributed sets of encrypted files between peers,
governed by build-in or custom Schemas.
Use this command to add, list, get, join, invite, and remove threads.
You can also stream/subscribe to thread updates.

Open threads are the most common thread type. Open threads allow 
any member to invite new members.

Private threads are primarily used internally for backup/recovery 
purpose and 1-to-1 communication channels.
`
}

func (x *threadsCmd) Shell() *ishell.Cmd {
	return nil
}

type addThreadsCmd struct {
	Client ClientOptions  `group:"Client Options"`
	Key    string         `short:"k" long:"key" description:"A locally unique key used by an app to identify this thread on recovery."`
	Open   bool           `short:"o" long:"open" description:"Set the thread type to open (default private)."`
	Schema flags.Filename `short:"s" long:"schema" description:"Thread Schema filename. Supercedes built-in schema flags."`
	Photos bool           `long:"photos" description:"Use the built-in photo Schema."`
}

func (x *addThreadsCmd) Name() string {
	return "add"
}

func (x *addThreadsCmd) Short() string {
	return "Add a new thread"
}

func (x *addThreadsCmd) Long() string {
	return "Adds and joins a new thread."
}

func (x *addThreadsCmd) Execute(args []string) error {
	setApi(x.Client)
	ttype := "private"
	if x.Open {
		ttype = "open"
	}

	var sch string
	if x.Schema == "" && x.Photos {
		sch = "photos"
	} else {
		sch = string(x.Schema)
	}

	opts := map[string]string{
		"key":    x.Key,
		"type":   ttype,
		"schema": sch,
	}
	return callAddThreads(args, opts)
}

func (x *addThreadsCmd) Shell() *ishell.Cmd {
	return nil
}

func callAddThreads(args []string, opts map[string]string) error {
	var body []byte

	sch := opts["schema"]
	if sch != "" && sch != "photos" {
		path, err := homedir.Expand(sch)
		if err != nil {
			path = sch
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		body, err = ioutil.ReadAll(file)

	} else if sch == "photos" {

		var node schema.Node
		if err := json.Unmarshal([]byte(textile.Photo), &node); err != nil {
			return err
		}
		var err error
		body, err = json.Marshal(&node)
		if err != nil {
			return err
		}
	}

	if body != nil {
		var schemaf *repo.File
		if _, err := executeJsonCmd(POST, "mills/schema", params{
			payload: bytes.NewReader(body),
			ctype:   "application/json",
		}, &schemaf); err != nil {
			return err
		}

		opts["schema"] = schemaf.Hash
	}

	var info *core.ThreadInfo
	res, err := executeJsonCmd(POST, "threads", params{args: args, opts: opts}, &info)
	if err != nil {
		return err
	}
	output(res, nil)
	return nil
}

type lsThreadsCmd struct {
	Client ClientOptions `group:"Client Options"`
}

func (x *lsThreadsCmd) Name() string {
	return "ls"
}

func (x *lsThreadsCmd) Short() string {
	return "List threads"
}

func (x *lsThreadsCmd) Long() string {
	return "List info about all threads."
}

func (x *lsThreadsCmd) Execute(args []string) error {
	setApi(x.Client)
	return callLsThreads()
}

func (x *lsThreadsCmd) Shell() *ishell.Cmd {
	return nil
}

func callLsThreads() error {
	var list *[]core.ThreadInfo
	res, err := executeJsonCmd(GET, "threads", params{}, &list)
	if err != nil {
		return err
	}
	output(res, nil)
	return nil
}

type getThreadsCmd struct {
	Client ClientOptions `group:"Client Options"`
}

func (x *getThreadsCmd) Name() string {
	return "get"
}

func (x *getThreadsCmd) Short() string {
	return "Get a thread"
}

func (x *getThreadsCmd) Long() string {
	return "Gets and displays info about a thread."
}

func (x *getThreadsCmd) Execute(args []string) error {
	setApi(x.Client)
	return callGetThreads(args, nil)
}

func (x *getThreadsCmd) Shell() *ishell.Cmd {
	return nil
}

func callGetThreads(args []string, ctx *ishell.Context) error {
	if len(args) == 0 {
		return errMissingThreadId
	}
	var info *core.ThreadInfo
	res, err := executeJsonCmd(GET, "threads/"+args[0], params{}, &info)
	if err != nil {
		return err
	}
	output(res, ctx)
	return nil
}

type getDefaultThreadsCmd struct {
	Client ClientOptions `group:"Client Options"`
}

func (x *getDefaultThreadsCmd) Name() string {
	return "default"
}

func (x *getDefaultThreadsCmd) Short() string {
	return "Get default thread"
}

func (x *getDefaultThreadsCmd) Long() string {
	return "Gets and displays info about the default thread (if selected)."
}

func (x *getDefaultThreadsCmd) Execute(args []string) error {
	setApi(x.Client)
	return callGetDefaultThreads()
}

func (x *getDefaultThreadsCmd) Shell() *ishell.Cmd {
	return nil
}

func callGetDefaultThreads() error {
	var info *core.ThreadInfo
	res, err := executeJsonCmd(GET, "threads/default", params{}, &info)
	if err != nil {
		return err
	}
	output(res, nil)
	return nil
}

type rmThreadsCmd struct {
	Client ClientOptions `group:"Client Options"`
}

func (x *rmThreadsCmd) Name() string {
	return "rm"
}

func (x *rmThreadsCmd) Short() string {
	return "Remove a thread"
}

func (x *rmThreadsCmd) Long() string {
	return "Leaves and removes a thread."
}

func (x *rmThreadsCmd) Execute(args []string) error {
	setApi(x.Client)
	return callRmThreads(args)
}

func (x *rmThreadsCmd) Shell() *ishell.Cmd {
	return nil
}

func callRmThreads(args []string) error {
	if len(args) == 0 {
		return errMissingThreadId
	}
	res, err := executeStringCmd(DEL, "threads/"+args[0], params{})
	if err != nil {
		return err
	}
	output(res, nil)
	return nil
}

type streamThreadsCmd struct {
	Client ClientOptions `group:"Client Options"`
	Events bool          `short:"e" long:"events" description:"Whether to return Server Sent Events (true) or JSON (default false)."`
}

func (x *streamThreadsCmd) Name() string {
	return "updates"
}

func (x *streamThreadsCmd) Short() string {
	return "Steam thread updates"
}

func (x *streamThreadsCmd) Long() string {
	return `
Streams all thread update types.
Use the --events option to emit Server-Sent Events (SSEvent), otherwise, emit JSON responses. 
SSEvents enable browsers/clients to consume the stream using EventSource.
`
}

func (x *streamThreadsCmd) Execute(args []string) error {
	setApi(x.Client)
	opts := map[string]string{"events": strconv.FormatBool(x.Events)}
	return callStreamThreads(args, opts, nil)
}

func (x *streamThreadsCmd) Shell() *ishell.Cmd {
	return nil // Don't support streaming api via shell
}

func callStreamThreads(args []string, opts map[string]string, ctx *ishell.Context) error {
	if len(args) == 0 {
		return errMissingThreadId
	}
	req, err := request(GET, "threads/"+args[0]+"/updates", params{opts: opts})
	if err != nil {
		return err
	}
	defer req.Body.Close()
	if req.StatusCode >= 400 {
		res, err := unmarshalString(req.Body)
		if err != nil {
			return err
		}
		return errors.New(res)
	}
	decoder := json.NewDecoder(req.Body)
	for {
		var info core.ThreadUpdate
		if err := decoder.Decode(&info); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		data, err := json.MarshalIndent(info, "", "    ")
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		output(string(data), ctx)
	}
	return nil
}

//////////////////////////////////////////////////////////////////////////////

//func listThreadPeers(c *ishell.Context) {
//	if len(c.Args) == 0 {
//		c.Err(errors.New("missing thread id"))
//		return
//	}
//	id := c.Args[0]
//
//	thrd := core.Node.Thread(id)
//	if thrd == nil {
//		c.Err(errors.New(fmt.Sprintf("could not find thread: %s", id)))
//		return
//	}
//
//	peers := thrd.Peers()
//	if len(peers) == 0 {
//		c.Println(fmt.Sprintf("no peers found in: %s", id))
//	} else {
//		c.Println(fmt.Sprintf("%v peers:", len(peers)))
//	}
//
//	green := color.New(color.FgHiGreen).SprintFunc()
//	for _, p := range peers {
//		c.Println(green(p.Id))
//	}
//}
//
//func listThreadBlocks(c *ishell.Context) {
//	if len(c.Args) == 0 {
//		c.Err(errors.New("missing thread id"))
//		return
//	}
//	threadId := c.Args[0]
//
//	thrd := core.Node.Thread(threadId)
//	if thrd == nil {
//		c.Err(errors.New(fmt.Sprintf("could not find thread: %s", threadId)))
//		return
//	}
//
//	blocks := core.Node.Blocks("", -1, "threadId='"+thrd.Id+"'")
//	if len(blocks) == 0 {
//		c.Println(fmt.Sprintf("no blocks found in: %s", threadId))
//	} else {
//		c.Println(fmt.Sprintf("%v blocks:", len(blocks)))
//	}
//
//	magenta := color.New(color.FgHiMagenta).SprintFunc()
//	for _, block := range blocks {
//		c.Println(magenta(fmt.Sprintf("%s %s", block.Type.Description(), block.Id)))
//	}
//}
//
//func ignoreBlock(c *ishell.Context) {
//	if len(c.Args) == 0 {
//		c.Err(errors.New("missing block id"))
//		return
//	}
//	id := c.Args[0]
//
//	block, err := core.Node.Block(id)
//	if err != nil {
//		c.Err(err)
//		return
//	}
//	thrd := core.Node.Thread(block.ThreadId)
//	if thrd == nil {
//		c.Err(errors.New(fmt.Sprintf("could not find thread %s", block.ThreadId)))
//		return
//	}
//
//	if _, err := thrd.Ignore(block.Id); err != nil {
//		c.Err(err)
//		return
//	}
//}
//
//func addThreadInvite(c *ishell.Context) {
//	if len(c.Args) == 0 {
//		c.Err(errors.New("missing peer id"))
//		return
//	}
//	peerId := c.Args[0]
//	if len(c.Args) == 1 {
//		c.Err(errors.New("missing thread id"))
//		return
//	}
//	threadId := c.Args[1]
//
//	thrd := core.Node.Thread(threadId)
//	if thrd == nil {
//		c.Err(errors.New(fmt.Sprintf("could not find thread: %s", threadId)))
//		return
//	}
//
//	pid, err := peer.IDB58Decode(peerId)
//	if err != nil {
//		c.Err(err)
//		return
//	}
//
//	if _, err := thrd.AddInvite(pid); err != nil {
//		c.Err(err)
//		return
//	}
//
//	green := color.New(color.FgHiGreen).SprintFunc()
//	c.Println(green("invite sent!"))
//}
//
//func acceptThreadInvite(c *ishell.Context) {
//	if len(c.Args) == 0 {
//		c.Err(errors.New("missing invite address"))
//		return
//	}
//	blockId := c.Args[0]
//
//	if _, err := core.Node.AcceptThreadInvite(blockId); err != nil {
//		c.Err(err)
//		return
//	}
//
//	green := color.New(color.FgHiGreen).SprintFunc()
//	c.Println(green("ok, accepted"))
//}
//
//func addExternalThreadInvite(c *ishell.Context) {
//	if len(c.Args) == 0 {
//		c.Err(errors.New("missing thread id"))
//		return
//	}
//	id := c.Args[0]
//
//	thrd := core.Node.Thread(id)
//	if thrd == nil {
//		c.Err(errors.New(fmt.Sprintf("could not find thread: %s", id)))
//		return
//	}
//
//	hash, key, err := thrd.AddExternalInvite()
//	if err != nil {
//		c.Err(err)
//		return
//	}
//
//	green := color.New(color.FgHiGreen).SprintFunc()
//	c.Println(green(fmt.Sprintf("added! creds: %s %s", hash.B58String(), string(key))))
//}
//
//func acceptExternalThreadInvite(c *ishell.Context) {
//	if len(c.Args) == 0 {
//		c.Err(errors.New("missing invite id"))
//		return
//	}
//	id := c.Args[0]
//	if len(c.Args) == 1 {
//		c.Err(errors.New("missing invite key"))
//		return
//	}
//	key := c.Args[1]
//
//	if _, err := core.Node.AcceptExternalThreadInvite(id, []byte(key)); err != nil {
//		c.Err(err)
//		return
//	}
//
//	green := color.New(color.FgHiGreen).SprintFunc()
//	c.Println(green("ok, accepted"))
//}
