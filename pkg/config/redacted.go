package config

const redactedSecretStr = "*** REDACTED ***"

// HideSensitiveInfo removes sensitive information from the config.
func HideSensitiveInfo(in Config) Config {
	out := in
	// TODO: avoid printing sensitive data without need to resetting them manually (which is an error-prone approach)
	for key, val := range out.Communications {
		val.SocketSlack.AppToken = redactedSecretStr
		val.SocketSlack.BotToken = redactedSecretStr
		val.Elasticsearch.Password = redactedSecretStr
		val.Discord.Token = redactedSecretStr
		val.Mattermost.Token = redactedSecretStr

		// maps are not addressable: https://stackoverflow.com/questions/42605337/cannot-assign-to-struct-field-in-a-map
		out.Communications[key] = val
	}

	return out
}
