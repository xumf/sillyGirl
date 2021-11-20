package wx

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/astaxie/beego/logs"
	"github.com/axgle/mahonia"
	"github.com/beego/beego/v2/adapter/httplib"
	"github.com/cdle/sillyGirl/core"
	"github.com/gin-gonic/gin"
)

var myip = ""
var relaier = wx.Get("relaier")
var mode = "bgm"

func init() {
	core.Pushs["wx"] = func(i interface{}, s string, _ interface{}) {
		if robot_wxid != "" {
			pmsg := TextMsg{
				Msg:    s,
				ToWxid: fmt.Sprint(i),
			}
			sendTextMsg(&pmsg)
		}
	}
	core.GroupPushs["wx"] = func(i, j interface{}, s string) {
		to := fmt.Sprint(i) + "@chatroom"
		pmsg := TextMsg{
			ToWxid: to,
		}
		if j != nil && fmt.Sprint(j) != "" {
			pmsg.MemberWxid = fmt.Sprint(j)
		}
		for _, v := range regexp.MustCompile(`\[CQ:image,file=([^\[\]]+)\]`).FindAllStringSubmatch(s, -1) {
			s = strings.Replace(s, fmt.Sprintf(`[CQ:image,file=%s]`, v[1]), "", -1)
			data, err := os.ReadFile("data/images/" + v[1])
			if err == nil {
				add := regexp.MustCompile("(https.*)").FindString(string(data))
				if add != "" {
					pmsg := OtherMsg{
						ToWxid: to,
						Msg: Msg{
							URL:  relay(add),
							Name: name(add),
						},
					}
					defer sendOtherMsg(&pmsg)
				}
			}
		}
		s = regexp.MustCompile(`\[CQ:([^\[\]]+)\]`).ReplaceAllString(s, "")
		pmsg.Msg = s
		sendTextMsg(&pmsg)
	}
	if wx.Get("vlw_addr") != "" {
		// go func() {
		// 	tosend = make(chan []byte, 10)
		// 	for {
		// 		time.Sleep(time.Microsecond * 200)
		// 		m := <-tosend
		// 		if c != nil {
		// 			c.WriteMessage(websocket.TextMessage, m)
		// 		} else {
		// 			time.Sleep(time.Second)
		// 			tosend <- m
		// 		}
		// 	}
		// }()
		// go enableVLW()
		mode = "vlw"
	}
	// else {
	// 	enableBGM()
	// }
	core.Server.POST("/wx/receive", func(c *gin.Context) {
		data, _ := c.GetRawData()
		if mode == "vlw" {
			type AutoGenerated struct {
				SdkVer  int    `json:"sdkVer"`
				Event   string `json:"Event"`
				Content struct {
					RobotWxid     string `json:"robot_wxid"`
					Type          int    `json:"type"`
					FromGroup     string `json:"from_group"`
					FromGroupName string `json:"from_group_name"`
					FromWxid      string `json:"from_wxid"`
					FromName      string `json:"from_name"`
					Msg           string `json:"msg"`
					MsgSource     struct {
						Atuserlist []struct {
							Wxid         string `json:"wxid"`
							Nickname     string `json:"nickname"`
							PositionFrom int    `json:"position_from"`
							PositionTo   int    `json:"position_to"`
						} `json:"atuserlist"`
					} `json:"msg_source"`
					Clientid  int `json:"clientid"`
					RobotType int `json:"robot_type"`
				} `json:"content"`
				// WsMCBreqID int `json:"wsMCBreqID"`
			}
			ag := &AutoGenerated{}
			json.Unmarshal(data, ag)
			if ag.Event == "EventPrivateChat" || ag.Event == "EventGroupChat" {
				wm := wxmsg{}
				wm.content = ag.Content.Msg
				if strings.Contains(wm.content, "<type>57</type>") {
					return
				}
				wm.user_id = ag.Content.FromWxid
				wm.user_name = ag.Content.FromName
				if ag.Content.FromGroup != "" {
					wm.chat_id = core.Int(strings.Replace(ag.Content.FromGroup, "@chatroom", "", -1))
				}
				if robot_wxid != ag.Content.RobotWxid {
					robot_wxid = ag.Content.RobotWxid
					wx.Set("robot_wxid", ag.Content.RobotWxid)
				}
				core.Senders <- &Sender{
					value: wm,
				}
			}
			logs.Info("recv: %s", data)
			return
		}
		jms := JsonMsg{}
		json.Unmarshal(data, &jms)
		c.JSON(200, map[string]string{"code": "-1"})
		fmt.Println(jms.Type, jms.Msg)
		if jms.Event != "EventFriendMsg" && jms.Event != "EventGroupMsg" {
			return
		}
		if jms.Type == 0 { //|| jms.Type == 49
			// if jms.Type != 1 && jms.Type != 3 && jms.Type != 5 {
			return
		}
		if strings.Contains(fmt.Sprint(jms.Msg), `<type>57</type>`) {
			return
		}
		if jms.FinalFromWxid == jms.RobotWxid {
			return
		}
		listen := wx.Get("onGroups")
		if jms.Event == "EventGroupMsg" && listen != "" && !strings.Contains(listen, strings.Replace(fmt.Sprint(jms.FromWxid), "@chatroom", "", -1)) {
			return
		}
		if robot_wxid != jms.RobotWxid {
			robot_wxid = jms.RobotWxid
			wx.Set("robot_wxid", robot_wxid)
		}
		if wx.GetBool("keaimao_dynamic_ip", false) {
			ip, _ := c.RemoteIP()
			wx.Set("api_url", fmt.Sprintf("http://%s:%s", ip.String(), wx.Get("keaimao_port", "8080"))) //
		}
		wm := wxmsg{}
		switch jms.Msg.(type) {
		case int, int64, int32:
			wm.content = fmt.Sprintf("%d", jms.Msg)
		case float64:
			wm.content = fmt.Sprintf("%d", int(jms.Msg.(float64)))
		default:
			wm.content = fmt.Sprint(jms.Msg)
		}
		wm.user_id = jms.FinalFromWxid
		wm.user_name = jms.FinalFromName
		if strings.Contains(jms.FromWxid, "@chatroom") {
			wm.chat_id = core.Int(strings.Replace(jms.FromWxid, "@chatroom", "", -1))
		}
		core.Senders <- &Sender{
			value: wm,
		}
	})
	core.Server.GET("/relay", func(c *gin.Context) {
		url := c.Query("url")
		rsp, err := httplib.Get(url).Response()
		if err == nil {
			io.Copy(c.Writer, rsp.Body)
		}
	})
	core.Server.GET("/wximage", func(c *gin.Context) {
		c.Writer.Write([]byte{})
	})
}

func relay(url string) string {
	if wx.GetBool("relay_mode", false) == false {
		return url
	}
	if mode == "vlw" {
		return url
	}
	if relaier != "" {
		return fmt.Sprintf(relaier, url)
	} else {
		if myip == "" || wx.GetBool("sillyGirl_dynamic_ip", false) == true {
			ip, _ := httplib.Get("https://imdraw.com/ip").String()
			if ip != "" {
				myip = ip
			}
		}
		return fmt.Sprintf("http://%s:%s/relay?url=%s", myip, wx.Get("relay_port", core.Bucket("sillyGirl").Get("port")), url) //"8002"
	}
}

func (sender *Sender) GetContent() string {
	if sender.Content != "" {
		return sender.Content
	}

	return sender.value.content
}
func (sender *Sender) GetUserID() string {
	return sender.value.user_id
}
func (sender *Sender) GetChatID() int {
	return sender.value.chat_id
}

func (sender *Sender) GetImType() string {
	return "wx"
}
func (sender *Sender) GetUsername() string {
	return sender.value.user_name
}
func (sender *Sender) GetReplySenderUserID() int {
	if !sender.IsReply() {
		return 0
	}
	return 0
}
func (sender *Sender) IsAdmin() bool {
	return strings.Contains(wx.Get("masters"), fmt.Sprint(sender.GetUserID()))
}
func (sender *Sender) Reply(msgs ...interface{}) (int, error) {
	to := ""
	if sender.value.chat_id != 0 {
		to = fmt.Sprintf("%d@chatroom", sender.value.chat_id)
	}
	at := ""
	if to == "" {
		to = sender.value.user_id
	} else {
		at = sender.value.user_id
	}
	pmsg := TextMsg{
		ToWxid:     to,
		MemberWxid: at,
	}
	for _, item := range msgs {
		switch item.(type) {
		case string:
			pmsg.Msg = item.(string)
			images := []string{}
			for _, v := range regexp.MustCompile(`\[CQ:image,file=base64://([^\[\]]+)\]`).FindAllStringSubmatch(pmsg.Msg, -1) {
				images = append(images, v[1])
				pmsg.Msg = strings.Replace(pmsg.Msg, fmt.Sprintf(`[CQ:image,file=base64://%s]`, v[1]), "", -1)
			}
			// for _, image := range images {
			// 	wxbase
			// }
		case []byte:
			pmsg.Msg = string(item.([]byte))
		case core.ImageUrl:
			url := string(item.(core.ImageUrl))
			pmsg := OtherMsg{
				ToWxid:     to,
				MemberWxid: at,
				Msg: Msg{
					URL:  relay(url),
					Name: name(url),
				},
			}
			sendOtherMsg(&pmsg)
		}
	}
	if pmsg.Msg != "" {
		sendTextMsg(&pmsg)
	}
	return 0, nil
}

func name(str string) string {
	pr := "jpg"
	ss := regexp.MustCompile(`\.([A-Za-z0-9]+)$`).FindStringSubmatch(str)
	if len(ss) != 0 {
		pr = ss[1]
	}
	md5 := md5V(str)
	return md5 + "." + pr
}

func md5V(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

func (sender *Sender) Copy() core.Sender {
	new := reflect.Indirect(reflect.ValueOf(interface{}(sender))).Interface().(Sender)
	return &new
}

func sendTextMsg(pmsg *TextMsg) {
	if mode == "vlw" {
		// if c == nil {
		// 	return
		// }
		type AutoGenerated struct {
			Token      string `json:"token"`
			API        string `json:"api"`
			RobotWxid  string `json:"robot_wxid"`
			ToWxid     string `json:"to_wxid"`
			Msg        string `json:"msg"`
			WsAPIreqID int    `json:"wsAPIreqID"`
		}
		a := AutoGenerated{}
		a.Token = wx.Get("vlw_token")
		a.API = "SendTextMsg"
		a.RobotWxid = robot_wxid
		a.ToWxid = pmsg.ToWxid
		a.Msg = pmsg.Msg
		vlw_addr := wx.Get("vlw_addr")
		data, _ := json.Marshal(a)
		req := httplib.Post(vlw_addr)
		req.Body(data)
		req.Response()
		// go func() {
		// 	tosend <- data
		// }()
		// c.WriteJSON(a)
	} else {
		if pmsg.Msg == "" {
			return
		}
		pmsg.Event = "SendTextMsg"
		pmsg.RobotWxid = robot_wxid
		req := httplib.Post(api_url())
		req.Header("Content-Type", "application/json")
		data, _ := json.Marshal(pmsg)
		enc := mahonia.NewEncoder("gbk")
		d := enc.ConvertString(string(data))
		d = regexp.MustCompile(`[\n\s]*\n[\s\n]*`).ReplaceAllString(d, "\n")
		req.Body(d)
		req.Response()
	}
}

func sendOtherMsg(pmsg *OtherMsg) {
	if pmsg.Event == "" {
		pmsg.Event = "SendImageMsg"
	}
	if mode == "vlw" {
		// if c == nil {
		// 	return
		// }
		type AutoGenerated struct {
			Token      string `json:"token"`
			API        string `json:"api"`
			RobotWxid  string `json:"robot_wxid"`
			ToWxid     string `json:"to_wxid"`
			Msg        string `json:"msg"`
			WsAPIreqID int    `json:"wsAPIreqID"`
			Path       string `json:"path"`
		}
		a := AutoGenerated{}
		a.Token = wx.Get("vlw_token")
		a.API = pmsg.Event
		a.RobotWxid = robot_wxid
		a.ToWxid = pmsg.ToWxid
		a.Path = pmsg.Msg.URL
		data, _ := json.Marshal(a)
		vlw_addr := wx.Get("vlw_addr")
		req := httplib.Post(vlw_addr)
		req.Body(data)
		req.Response()
		// go func() {
		// 	tosend <- data
		// }()
	} else {
		pmsg.RobotWxid = robot_wxid
		req := httplib.Post(api_url())
		req.Header("Content-Type", "application/json")
		data, _ := json.Marshal(pmsg)
		req.Body(data)
		req.Response()
	}
}
