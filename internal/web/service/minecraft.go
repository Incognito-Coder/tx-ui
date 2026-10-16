package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"x-ui/internal/util/common"
)

// The XMC finalmask disguises a connection as a Minecraft login, and both ends
// need the same signed profile for it: Mojang's session server hands out the
// textures and their signature, which cannot be made up.
var (
	mojangProfileURL = "https://api.mojang.com/users/profiles/minecraft/"
	mojangSessionURL = "https://sessionserver.mojang.com/session/minecraft/profile/"

	minecraftUsername = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)
)

type MinecraftProfile struct {
	Username          string `json:"username"`
	UUID              string `json:"uuid"`
	TexturesValue     string `json:"texturesValue"`
	TexturesSignature string `json:"texturesSignature"`
}

func (s *ServerService) GetMinecraftProfile(username string) (*MinecraftProfile, error) {
	if !minecraftUsername.MatchString(username) {
		return nil, common.NewErrorf("invalid Minecraft username: %q", username)
	}
	client := &http.Client{Timeout: 10 * time.Second}

	var account struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := getMojangJSON(client, mojangProfileURL+url.PathEscape(username), &account); err != nil {
		return nil, err
	}
	if len(account.ID) != 32 {
		return nil, common.NewErrorf("no Minecraft account named %q", username)
	}

	var session struct {
		Name       string `json:"name"`
		Properties []struct {
			Name      string `json:"name"`
			Value     string `json:"value"`
			Signature string `json:"signature"`
		} `json:"properties"`
	}
	if err := getMojangJSON(client, mojangSessionURL+account.ID+"?unsigned=false", &session); err != nil {
		return nil, err
	}
	for _, property := range session.Properties {
		if property.Name == "textures" && property.Value != "" && property.Signature != "" {
			id := account.ID
			return &MinecraftProfile{
				Username:          session.Name,
				UUID:              fmt.Sprintf("%s-%s-%s-%s-%s", id[:8], id[8:12], id[12:16], id[16:20], id[20:]),
				TexturesValue:     property.Value,
				TexturesSignature: property.Signature,
			}, nil
		}
	}
	return nil, common.NewErrorf("the profile of %q has no signed textures", username)
}

func getMojangJSON(client *http.Client, address string, into any) error {
	resp, err := client.Get(address)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent:
		return common.NewError("no such Minecraft account")
	case resp.StatusCode != http.StatusOK:
		return common.NewErrorf("Mojang answered %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}
