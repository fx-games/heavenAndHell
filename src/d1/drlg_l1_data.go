package d1

const (
	t1VWall          = 1
	t1HWall          = 2
	t1Corner         = 3
	t1DWall          = 4
	t1DArch          = 5
	t1VWallEnd       = 6
	t1HWallEnd       = 7
	t1HArchEnd       = 8
	t1VArchEnd       = 9
	t1HArchVWall     = 10
	t1VArch          = 11
	t1HArch          = 12
	t1Floor          = 13
	t1HWallVArch     = 14
	t1Pillar         = 15
	t1VCorner        = 16
	t1HCorner        = 17
	t1DirtHwall      = 18
	t1DirtVwall      = 19
	t1VDirtCorner    = 20
	t1HDirtCorner    = 21
	t1Dirt           = 22
	t1DirtHwallEnd   = 23
	t1DirtVwallEnd   = 24
	t1VDoor          = 25
	t1HDoor          = 26
	t1HFenceVWall    = 27
	t1HDoorVDoor     = 28
	t1DFence         = 29
	t1VDoorEnd       = 30
	t1HDoorEnd       = 31
	t1VFenceEnd      = 32
	t1VArchEnd2      = 33
	t1HArchVWall2    = 34
	t1VFence         = 35
	t1HFence         = 36
	t1HWallVFence    = 37
	t1HArchVFence    = 38
	t1HArchVDoor     = 39
	t1HArchVWall3    = 40
	t1DWall2         = 41
	t1HWallVArch2    = 42
	t1DWall3         = 43
	t1EntranceStairs = 64
	t1VWall2         = 79
	t1HWall2         = 80
	t1DWall4         = 82
	t1VWallEnd2      = 84
	t1VWall4         = 89
	t1VWall5         = 90
	t1HWall4         = 91
	t1HWall5         = 92
	t1VWall8         = 100
	t1Floor12        = 139
	t1Floor13        = 140
	t1Floor14        = 141
	t1Floor15        = 142
	t1Floor16        = 143
	t1Floor17        = 144
	t1Floor18        = 145
	t1VWall17        = 146
	t1VArch5         = 147
	t1HWallShadow    = 148
	t1HArchShadow    = 149
	t1Floor19        = 150
	t1Floor20        = 151
	t1Floor21        = 152
	t1HArchShadow2   = 153
	t1HWallShadow2   = 154
	t1Floor22        = 162
	t1Floor23        = 163
	t1DirtHWall2     = 199
	t1DirtVWall2     = 200
	t1DirtCorner2    = 202
	t1DirtHWallEnd2  = 204
	t1DirtVWallEnd2  = 205
)

// l1ShadowPatterns: strig, s1, s2, s3, nv1, nv2, nv3.
var l1ShadowPatterns = [37][7]byte{
	{t1HWallEnd, t1Floor, 0, t1Floor, t1Floor17, 0, t1Floor15},
	{t1VCorner, t1Floor, 0, t1Floor, t1Floor17, 0, t1Floor15},
	{t1Pillar, t1Floor, 0, t1Floor, t1Floor18, 0, t1Floor15},
	{t1DArch, t1Floor, t1Floor, t1Floor, t1Floor21, t1Floor13, t1Floor12},
	{t1DArch, t1Floor, t1VWall, t1Floor, t1Floor16, t1VWall17, t1Floor12},
	{t1DArch, t1Floor, t1Floor, t1HWall, t1Floor16, t1Floor13, t1HWallShadow},
	{t1DArch, 0, t1VWall, t1HWall, 0, t1VWall17, t1HWallShadow},
	{t1DArch, t1Floor, t1VArch, t1Floor, t1Floor16, t1VArch5, t1Floor12},
	{t1DArch, t1Floor, t1Floor, t1HArch, t1Floor16, t1Floor13, t1HArchShadow},
	{t1DArch, t1Floor, t1VArch, t1HArch, t1Floor19, t1VArch5, t1HArchShadow},
	{t1DArch, t1Floor, t1VWall, t1HArch, t1Floor16, t1VWall17, t1HArchShadow},
	{t1DArch, t1Floor, t1VArch, t1HWall, t1Floor16, t1VArch5, t1HWallShadow},
	{t1VArchEnd, t1Floor, t1Floor, t1Floor, t1Floor17, t1Floor13, t1Floor15},
	{t1VArchEnd, t1Floor, t1VWall, t1Floor, t1Floor17, t1VWall17, t1Floor15},
	{t1VArchEnd, t1Floor, t1VArch, t1Floor, t1Floor20, t1VArch5, t1Floor15},
	{t1HArchEnd, t1Floor, 0, t1Floor, t1Floor17, 0, t1Floor12},
	{t1HArchEnd, t1Floor, 0, t1HArch, t1Floor16, 0, t1HArchShadow},
	{t1HArchEnd, 0, 0, t1HWall, 0, 0, t1HWallShadow},
	{t1VArch, 0, 0, t1Floor, 0, 0, t1Floor12},
	{t1VArch, t1Floor, 0, t1Floor, t1Floor12, 0, t1Floor12},
	{t1VArch, t1HWall, 0, t1Floor, t1HWallShadow, 0, t1Floor12},
	{t1VArch, t1HArch, 0, t1Floor, t1HArchShadow, 0, t1Floor12},
	{t1VArch, t1Floor, t1VArch, t1HArch, t1Floor12, 0, t1HArchShadow},
	{t1HWallVArch, 0, 0, t1Floor, 0, 0, t1Floor12},
	{t1HWallVArch, t1Floor, 0, t1Floor, t1Floor12, 0, t1Floor12},
	{t1HWallVArch, t1HWall, 0, t1Floor, t1HWallShadow, 0, t1Floor12},
	{t1HWallVArch, t1HArch, 0, t1Floor, t1HArchShadow, 0, t1Floor12},
	{t1HWallVArch, t1Floor, t1VArch, t1HArch, t1Floor12, 0, t1HArchShadow},
	{t1HArchVWall, 0, t1Floor, 0, 0, t1Floor13, 0},
	{t1HArchVWall, t1Floor, t1Floor, 0, t1Floor13, t1Floor13, 0},
	{t1HArchVWall, 0, t1VWall, 0, 0, t1VWall17, 0},
	{t1HArchVWall, t1Floor, t1VArch, 0, t1Floor13, t1VArch5, 0},
	{t1HArch, 0, t1Floor, 0, 0, t1Floor13, 0},
	{t1HArch, t1Floor, t1Floor, 0, t1Floor13, t1Floor13, 0},
	{t1HArch, 0, t1VWall, 0, 0, t1VWall17, 0},
	{t1HArch, t1Floor, t1VArch, 0, t1Floor13, t1VArch5, 0},
	{t1Corner, t1Floor, t1VArch, t1HArch, t1Floor19, 0, 0},
}

// l1BaseTypes maps tile IDs to their base tile.
var l1BaseTypes = [207]byte{
	0,
	t1VWall, t1HWall, t1Corner, t1DWall, t1DArch, t1VWallEnd, t1HWallEnd, t1HArchEnd, t1VArchEnd,
	t1HArchVWall, t1VArch, t1HArch, t1Floor, t1HWallVArch, t1Pillar, t1VCorner, t1HCorner,
	0, 0, 0, 0, 0, 0, 0,
	t1VWall, t1HWall, t1HArchVWall, t1DWall, t1DArch, t1VWallEnd, t1HWallEnd, t1HArchEnd,
	t1VArchEnd, t1HArchVWall, t1VArch, t1HArch, t1HWallVArch, t1DArch, t1HWallVArch,
	t1HArchVWall, t1DWall, t1HWallVArch, t1DWall, t1DArch,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0,
	t1VWall, t1HWall, t1Corner, t1DWall, t1VWall, t1VWallEnd, t1HWallEnd, t1VCorner, t1HCorner,
	t1HWall, t1VWall, t1VWall, t1HWall, t1HWall, t1VWall, t1VWall, t1HWall, t1HWall, t1HWall, t1HWall,
	t1HWall, t1VWall, t1VWall, t1VArch, t1VWall, t1Floor, t1Floor, t1Floor, t1VWall, t1HWall, t1VWall,
	t1HWall, t1VWall, t1HWall, t1VWall, t1HWall, t1HWall, t1HWall, t1HWall, t1HArch,
	0, 0,
	t1VArch, t1VWall, t1VArch, t1VWall, t1Floor,
	0, 0, 0, 0, 0, 0, 0,
	t1Floor, t1Floor, t1Floor, t1Floor, t1Floor, t1Floor, t1Floor, t1Floor, t1Floor, t1Floor, t1Floor,
	t1Floor, t1Floor, t1VWall, t1VArch, t1HWall, t1HArch, t1Floor, t1Floor, t1Floor, t1HArch, t1HWall,
	t1VWall, t1HWall, t1HWall, t1DWall, t1HWallVArch, t1DWall, t1HArchVWall, t1Floor, t1Floor,
	t1DWall, t1DWall, t1VWall, t1VWall, t1DWall, t1HWall, t1HWall, t1Floor, t1Floor, t1Floor, t1Floor,
	t1VDoor, t1HDoor, t1HDoorVDoor, t1VDoorEnd, t1HDoorEnd, t1DWall2, t1DWall3, t1HArchVWall3,
	t1DWall2, t1HWallVArch2, t1DWall3, t1VDoor, t1DWall2, t1DWall3, t1HDoorVDoor, t1HDoorVDoor,
	t1VWall, t1HWall, t1VDoor, t1HDoor, t1Dirt, t1Dirt, t1VDoor, t1HDoor,
	0, 0, 0, 0, 0, 0, 0, 0,
}

// l1TileDecorations maps tile IDs to their undecorated tile.
var l1TileDecorations = [207]byte{
	0,
	t1VWall, t1HWall, t1Corner, t1DWall, t1DArch, t1VWallEnd, t1HWallEnd, t1HArchEnd, t1VArchEnd,
	t1HArchVWall, t1VArch, t1HArch, t1Floor, t1HWallVArch, t1Pillar, t1VCorner, t1HCorner,
	0, 0, 0, 0, 0, 0, 0,
	t1VDoor, t1HDoor,
	0,
	t1HDoorVDoor,
	0,
	t1VDoorEnd, t1HDoorEnd,
	0, 0, 0, 0, 0, 0, 0, 0,
	t1HArchVWall3, t1DWall2, t1HWallVArch2, t1DWall3,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	t1VWall2, t1HWall2,
	0,
	t1DWall4,
	0, 0, 0, 0, 0, 0,
	t1VWall2,
	0,
	t1HWall2,
	0, 0,
	t1VWall2, t1HWall2,
	0,
	t1HWall, t1HWall, t1HWall, t1VWall, t1VWall, t1VArch, t1VDoor, t1Floor, t1Floor, t1Floor, t1VWall,
	t1HWall, t1VWall, t1HWall, t1VWall, t1HWall, t1VWall, t1HWall, t1HWall, t1HWall, t1HWall, t1HArch,
	0, 0,
	t1VArch, t1VWall, t1VArch, t1VWall, t1Floor,
	0, 0, 0, 0, 0, 0, 0,
	t1Floor, t1Floor, t1Floor, t1Floor, t1Floor, t1Floor,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}

var (
	l1STAIRSUP   = miniset{4, 4, [6][6]byte{{13, 13, 13, 13}, {2, 2, 2, 2}, {13, 13, 13, 13}, {13, 13, 13, 13}}, [6][6]byte{{0, 66, 6, 0}, {63, 64, 65, 0}, {0, 67, 68, 0}, {0, 0, 0, 0}}}
	l1STAIRSDOWN = miniset{4, 3, [6][6]byte{{13, 13, 13, 13}, {13, 13, 13, 13}, {13, 13, 13, 13}}, [6][6]byte{{62, 57, 58, 0}, {61, 59, 60, 0}, {0, 0, 0, 0}}}
	l1L5STAIRSUP = miniset{4, 4, [6][6]byte{{22, 22, 22, 22}, {2, 2, 2, 2}, {13, 13, 13, 13}, {13, 13, 13, 13}}, [6][6]byte{{0, 66, 23, 0}, {63, 64, 65, 0}, {0, 67, 68, 0}, {0, 0, 0, 0}}}
	l1LAMPS      = miniset{2, 2, [6][6]byte{{13, 0}, {13, 13}}, [6][6]byte{{129, 0}, {130, 128}}}
	l1PWATERIN   = miniset{6, 6, [6][6]byte{{13, 13, 13, 13, 13, 13}, {13, 13, 13, 13, 13, 13}, {13, 13, 13, 13, 13, 13}, {13, 13, 13, 13, 13, 13}, {13, 13, 13, 13, 13, 13}, {13, 13, 13, 13, 13, 13}}, [6][6]byte{{0, 0, 0, 0, 0, 0}, {0, 202, 200, 200, 84, 0}, {0, 199, 203, 203, 83, 0}, {0, 85, 206, 80, 81, 0}, {0, 0, 134, 135, 0, 0}, {0, 0, 0, 0, 0, 0}}}
)
