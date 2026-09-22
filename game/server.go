package game

import (
	"cmp"
	"fmt"
	ipc "github.com/james-barrow/golang-ipc"
	"rpg/common"
	"rpg/defs"
	"rpg/util"
	"slices"
	"strings"
)

type TextServer struct {
	server      *ipc.Server
	socket      string
	messageType int
}

func NewTextServer(socket string, msgType int) *TextServer {
	server, _ := ipc.StartServer(socket, nil)
	return &TextServer{
		socket:      socket,
		messageType: msgType,
		server:      server,
	}
}

func (s *TextServer) Write(text string) {
	if s.server == nil || (s.server.StatusCode() != ipc.Connected && s.server.StatusCode() != ipc.Listening) {
		s.server, _ = ipc.StartServer(s.socket, nil)
	}
	if s.server.StatusCode() == ipc.Connected {
		err := s.server.Write(s.messageType, []byte(text))
		if err != nil {
			println(fmt.Sprintf("error writing to log server: %v", err))
			return
		}
	}
}

// startWindowServers opens the IPC text windows (log, stats, inventory, equipment) enabled in the config.
func (g *GameInstance) startWindowServers() {
	if g.configuration.ServeLog {
		g.serverLog = NewTextServer("rogue_log", 1)
		g.SetLogHook(g.serverLog.Write)
	}
	if g.configuration.ServeStats {
		g.serverStats = NewTextServer("rogue_stats", 2)
	}
	if g.configuration.ServeInventory {
		g.serverInventory = NewTextServer("rogue_inventory", 3)
	}
	if g.configuration.ServeEquipment {
		g.serverEquipment = NewTextServer("rogue_equipment", 4)
	}
}

func (g *GameInstance) sendStatsToWindowClient() {
	if !g.configuration.ServeStats {
		return
	}
	lines := g.GetWorldState().GetPlayer().GetFullStats()
	var asStrings []string
	for _, line := range lines {
		asStrings = append(asStrings, line.Text)
	}
	g.serverStats.Write(strings.Join(asStrings, "\n"))
}

func (g *GameInstance) sendInventoryToWindowClient() {
	if !g.configuration.ServeInventory {
		return
	}
	asStrings := g.state.GetPlayer().GetInventory().AsColorCodedStrings()
	g.serverInventory.Write(strings.Join(asStrings, "\n"))
}

func (g *GameInstance) sendEquipmentToWindowClient() {
	if !g.configuration.ServeEquipment {
		return
	}
	equipment := g.state.GetPlayer().GetEquipment()
	inventory := g.state.GetPlayer().GetInventory()
	items := sortEquipForUI(equipment)
	var asStrings []string

	maxItemLength := 0
	maxSlotLength := 0
	for _, item := range items {
		// index 0 => a, 1 => b, 2 => c, ...
		//itemColor := common.ColorCodeFromColor(item.TintColor())
		slotName := equipment.GetSlot(item)
		slotDesc := fmt.Sprintf("<-- %s", slotName.ToString())

		//itemLine := fmt.Sprintf("[:white]%c) %s%s", 'a'+index, itemColor, item.Name())
		shortcut := inventory.GetLetterForItem(item)
		itemLineWithoutColor := fmt.Sprintf("%c) %s", shortcut, item.InventoryName())
		//minLineLength := len(itemLineWithoutColor) + len(slotDesc) + 1

		//asStrings = append(asStrings, itemLine)
		if len(itemLineWithoutColor) > maxItemLength {
			maxItemLength = len(itemLineWithoutColor)
		}
		if len(slotDesc) > maxSlotLength {
			maxSlotLength = len(slotDesc)
		}
	}

	for _, item := range items {
		// index 0 => a, 1 => b, 2 => c, ...
		itemColor := defs.ColorCodeFromColor(item.TintColor())

		slotName := equipment.GetSlot(item)
		slotDesc := fmt.Sprintf("[:white]<-- %s", slotName.ToString())
		shortcut := inventory.GetLetterForItem(item)
		itemLine := fmt.Sprintf("[:white]%c) %s%s", shortcut, itemColor, item.InventoryName())

		withoutColor := common.RemoveColorCodes(slotDesc)
		padForSlot := (maxSlotLength + 1) - len(withoutColor)
		padForItem := maxItemLength - len(common.RemoveColorCodes(itemLine))
		slotDesc = util.LeftPadCount(slotDesc, padForSlot)
		itemLine = util.RightPadCount(itemLine, padForItem)

		asStrings = append(asStrings, itemLine+slotDesc)
	}
	g.serverEquipment.Write(strings.Join(asStrings, "\n"))
}
func sortEquipForUI(equip *common.Equipment) []*common.Item {
	itemList := equip.AllItems()

	slices.SortStableFunc(itemList, func(i, j *common.Item) int {
		slotNameI := equip.GetSlot(i)
		slotNameJ := equip.GetSlot(j)
		return cmp.Compare(slotNameI, slotNameJ)
	})
	return itemList
}
