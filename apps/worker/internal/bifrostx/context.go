package bifrostx

import (
	schemas "github.com/maximhq/bifrost/core/schemas"
)

func SetRequestSelection(ctx *schemas.BifrostContext, channelID, keyID int, keyValue, model string) {
	if ctx == nil {
		return
	}
	ctx.SetValue(contextKeyChannelID, channelID)
	ctx.SetValue(contextKeySelectedKeyID, keyID)
	ctx.SetValue(contextKeySelectedKey, keyValue)
	ctx.SetValue(contextKeySelectedModel, model)
}
