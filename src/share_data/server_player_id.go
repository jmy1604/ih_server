package share_data

func GetServerIdByPlayerId(player_id int32) int32 {
	return (player_id >> 20) & 0xffff
}

func GeneratePlayerId(server_id, serial_id int32) int32 {
	return ((server_id << 20) & 0x7ff00000) | serial_id
}

func GetServerIdByGuildId(guild_id int32) int32 {
	return (guild_id >> 20) & 0xffff
}

func GenerateGuildId(server_id, serial_id int32) int32 {
	return ((server_id << 20) & 0x7ff00000) | serial_id
}
