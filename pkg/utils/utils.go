package utils

import "path"

// BuildFinalPath format: CopyToPath.MainPath/CopyToPath.SubPath/RoomId
func BuildFinalPath(mainPath, subPath, roomId string) string {
	return path.Join(mainPath, subPath, roomId)
}
