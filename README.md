# Rediscord

SA:MP to Discord plugin built with Redis as the bridge.


## Setup

Ensure you have access to a Redis server and you have the Redis plugin installed on your SA:MP server.

- `go get github.com/Southclaws/samp-go`
- `go get github.com/Southclaws/samp-rediscord`
- `cd $GOPATH/src/github.com/Southclaws/samp-rediscord`
- `go build` or `go install` whatever floats your boat
- put the binary where you want, set up a systemd/upstart/etc entry
- create a `config.json` next to the binary
- run the binary

## Configuration

In your Pawn code, you'll need to set up the communication to the app via Redis list-queues:

### Outgoing chat

Outgoing chat is chat going *out* of the game server. This should be done with a `Redis_SendMessage` call with the following queue name: `<domain>.rediscord.<target>` where `<domain>` is your `domain` setting in `config.json` and `<target>` is a `output_queue` entry in the `discord_bind` array in `config.json`.

Note: Regarding the `domain`, it is generally a good practice to keep all your Redis keys named with a prefix and period delimiter similar to web domains.

The actual message text should be in the format:

`<player info>:<message>`

You can place anything in the info section, player name, ID, faction name, admin status, etc. but you must use the colon delimiter to separate the player info from the message they are sending.

```pawn
public OnPlayerText(playerid, text[]) // or hook OnPlayerText(playerid, text[]) if you're into that
{
    Redis_SendMessage(gRedis,
        "myserver.rediscord.outgoing",
        sprintf("%p (%d):%s", playerid, playerid, text));
}
```

If you want to have multiple chats, use different 

### Incoming chat

Incoming chat is chat coming *in* to the game server from a Discord channel. You can get this into your game server by binding a Pawn callback to a Redis list queue:

```pawn
forward OnDiscordChat(data[]);

public OnGameModeInit() // or hook OnGameModeInit() if you're into that
{
	Redis_BindMessage(gRedis, "myserver.rediscord.incoming", "OnDiscordChat");
}

public OnDiscordChat(data[])
{
    SendClientMessageToAll(data);
}
```

Messages come in the `data` field of the event you register and they are in the format: `<username>:<message>` so you can split on the first colon and present your messages in a nicer way if you want.

### Example `config.json`:

```json
{
    "redis_auth": "password",
    "redis_host": "my.redis.server",
    "redis_port": "6379",
    "redis_dbid": 10,
    "domain": "myserver",
    "discord_token": "discord-app-token",
    "discord_binds": [
        {  
            "channel_id": "discord channel id",
            "input_queue": "incoming-main-chat",
            "output_queue": "outgoing-main-chat"
        },
        {
            "channel_id": "discord channel id",
            "input_queue": "incoming-admin-chat",
            "output_queue": "outgoing-admin-chat"
        }
    ]
}
```