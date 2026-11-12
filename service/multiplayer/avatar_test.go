package multiplayer

// 文件职责：验证头像增量的旧帧兼容、事件解码及状态投影。
// 使用：本机 go test，不启动网络或读取真实账户；边界：不证明生产图片服务可用。

import (
	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"
	"encoding/json"
	"testing"
)

// TestAvatarCompatibility 职责：验证旧服务缺字段、新字段及未知扩展；使用：回归契约；边界：不访问图片地址。
func TestAvatarCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field string
		want  string
	}{
		{"legacy", "", ""},
		{"null", `,"headerImg":null`, ""},
		{"empty", `,"headerImg":""`, ""},
		{"relative", `,"headerImg":"/uploads/a.png"`, "/uploads/a.png"},
		{"absolute", `,"headerImg":"https://images.invalid/a.png","futureField":1`, "https://images.invalid/a.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var user mpAPI.UserInfo
			var host mpAPI.RoomHost
			var member mpAPI.Member
			raw := []byte(`{"userId":2,"nickname":"测试"` + tc.field + `}`)
			for _, out := range []any{&user, &host, &member} {
				if err := json.Unmarshal(raw, out); err != nil {
					t.Fatal(err)
				}
			}
			if user.HeaderImg != tc.want || host.HeaderImg != tc.want || member.HeaderImg != tc.want {
				t.Fatalf("avatar mismatch: user=%q host=%q member=%q", user.HeaderImg, host.HeaderImg, member.HeaderImg)
			}
		})
	}
}

// TestAvatarEventProjection 职责：验证加入、链路快照、大厅替换和账户头像进入前端状态；使用：无网络控制器；边界：不创建数据面。
func TestAvatarEventProjection(t *testing.T) {
	c := newController(options{token: func() string { return "" }})
	sess := &session{info: mpAPI.SessionOpenReply{User: mpAPI.UserInfo{UserID: 2, HeaderImg: "/self.png"}}}
	c.sess = sess
	c.room = &roomState{view: RoomView{Role: mpConstant.RoleMember, Snapshot: mpAPI.RoomSnapshot{RoomID: "r_test", Revision: 1}}}
	join := json.RawMessage(`{"roomId":"r_test","result":0,"member":{"userId":3,"headerImg":"/new.png"},"room":{"roomId":"r_test","revision":2,"host":{"userId":1,"headerImg":"/host.png"},"members":[{"userId":2,"headerImg":"/self.png"},{"userId":3,"headerImg":"/new.png"}]}}`)
	code, _, changed := c.applyEvent(sess, &mpAPI.ActiveFrame{Opcode: mpConstant.EvMemberJoinAfter, Data: join})
	if code != 0 || !changed {
		t.Fatalf("join: code=%d changed=%v", code, changed)
	}
	state := c.snapshotState()
	if state.Account.HeaderImg != "/self.png" || state.Room.Snapshot.Host.HeaderImg != "/host.png" || state.Room.Snapshot.Members[1].HeaderImg != "/new.png" {
		t.Fatalf("join projection lost avatars: %+v", state)
	}
	// 旧版本快照不能覆盖当前图；更新快照的空值必须清除旧图。
	for _, revision := range []int{1, 2, 3} {
		raw, err := json.Marshal(map[string]any{"roomId": "r_test", "room": map[string]any{"roomId": "r_test", "revision": revision, "host": map[string]any{"userId": 1}}})
		if err != nil {
			t.Fatal(err)
		}
		code, _, _ = c.applyEvent(sess, &mpAPI.ActiveFrame{Opcode: mpConstant.EvMemberLinkAfter, Data: raw})
		if code != 0 {
			t.Fatal(code)
		}
		want := "/host.png"
		if revision == 3 {
			want = ""
		}
		if got := c.snapshotState().Room.Snapshot.Host.HeaderImg; got != want {
			t.Fatalf("revision %d: %q != %q", revision, got, want)
		}
	}
	// 大厅按 roomId 的 upsert 必须带头像替换；后续旧格式/空值覆盖不能保留旧头像。
	for _, field := range []string{`,"hostHeaderImg":"/host.png"`, `,"hostHeaderImg":"/changed.png"`, ``} {
		raw := json.RawMessage(`{"upserted":[{"roomId":"r_test","hostNickname":"房主"` + field + `}],"removed":[]}`)
		code, _, changed = c.applyEvent(sess, &mpAPI.ActiveFrame{Opcode: mpConstant.EvLobbyChangeAfter, Data: raw})
		if code != 0 || !changed {
			t.Fatalf("lobby: %d %v", code, changed)
		}
		projected, err := json.Marshal(c.snapshotState())
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip State
		if err = json.Unmarshal(projected, &roundTrip); err != nil {
			t.Fatal(err)
		}
		var event mpAPI.LobbyChangeEvent
		if err = json.Unmarshal(raw, &event); err != nil {
			t.Fatal(err)
		}
		if len(roundTrip.Lobby.List) != 1 || roundTrip.Lobby.List[0].HostHeaderImg != event.Upserted[0].HostHeaderImg {
			t.Fatalf("lobby lost/reused image: %s", projected)
		}
	}
}
