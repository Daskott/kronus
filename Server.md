## Kronus `server` (pre-alpha)

The `server` is a new* feature currently in `pre-alpha`, and available through the `server` cmd on the CLI app.
```
kronus server -h
```
Kronus server allows you to schedule a liveliness probe for users on the server and sends out messages to emergency contacts if no/bad response is gotten from a user.

### Dependcies
- Google cloud storage credentials - (Optional - for db backup)
- Twilio account - for sending probe messages

### Server Config
The server requires a valid `/config.yml` configuration file as shown below:
```yml
kronus:
  # A valid RSA private key for creating/validating kronus server jwts
  # You can can use this https://mkjwk.org/ to generate one
  privateKeyPem:

  # If deployed to a production env, the public url of the server.
  # It's required, as this will be used for the twilio webhook 
  publicUrl: "https://my-app.com"
  
  cron:
    timeZone: "America/Toronto" # Timezone to use for scheduling probes
  
  listener:
    port: 3000

sqlite:
  passPhrase: passphrase

google:
  storage:
    bucket: "gstorage-bucket-name"

    # The folder/path to store backup files
    prefix: "kronus"
    
    # How often you want your sqlite db to be backed up to google storage in cron format
    sqliteBackupSchedule: "0 * * * *"

    enableSqliteBackupAndSync: true
  applicationCredentials: "/path/to/google/credentials"

twilio:
  accountSid: AHKX00SXXXXXXXXXXXXXXXXXXX
  authToken: BHKX00SXXXXXXXXXXXXXXXXXXX
  messagingServiceSid: CHKX00SXXXXXXXXXXXXXXXXXXX
```

### Start server
```
kronus server --config=/config.yml
```

### API and Usage
- `POST` **/users**  - The first user created is assigned `admin` role & every other user has to be created by the `admin`
```json
{
    "first_name": "tony",
    "last_name": "stark",
    "email": "stark@avengers.com",
    "password": "very-secure",
    "phone_number": "+12345678900"
}
```

- `POST` **/login**  - Login to get a `token` which will be used to query protected resources 
```json
{
    "email": "stark@avengers.com",
    "password": "very-secure"
}
```
- `POST` **/users/{uid}/contacts/** - For protected routes, the token from the provious step needs to be added to the `Authorization` header as `Bearer <TOKEN>`
```json
{
    "first_name": "strongest",
    "last_name": "avenger",
    "phone_number": "+12345678900",
    "email": "hulk@avengers.com",
    "is_emergency_contact": true
}
```

- `PUT` **/users/{uid}/probe_settings/** set `day` and `time` of the week to receive probe messages & use `active` to enable/disable probe.
```json
{
    "day": "fri",
    "time": "22:00",
    "active": true
}
```
- Admin only routes

    | Method | Route | Note |
    | --- | --- | --- |
    | `GET` | **/users** | Fetch all users |
    | `GET` | **/jobs/stats** | Get job stats i.e. no of jobs in each group e.g. `enqueued`, `successful`, `in-progress` or `dead` |
    | `GET` | **/jobs?status=** | Fetch jobs by status where status could be `enqueued`, `successful`, `in-progress` or `dead` |
    | `GET` | **/probes/stats** | Get job stats i.e. no of probes in each group e.g. `pending`, `good`, `bad` `cancelled`, or `unavailable`|
    | `GET` | **/probes?status=** | Fetch probes by status where status could be  `pending`, `good`, `bad` `cancelled`, or `unavailable` |

- Other routes

    | Method | Route | Note |
    | --- | --- | --- |
    | `POST` | **/webhook/sms** | For twilio message webhook |
    | `GET` | **/jwks** | For validating kronus server jwts |
    | `GET` | **/health** | To check service health |
    | `GET` | **/users/{uid}**| Can only GET your own record, except if you're admin |
    | `PUT` |**/users/{uid}**| Can only UPDATE own record |
    | `DELETE` |**/users/{uid}**| Can only DELETE your own record, except if you're admin |
    | `GET` |**/users/{uid}/contacts**| Fetch all contacts for the given user id i.e. `uid` |
    | `PUT` |**/users/{uid}/contacts/{id}**| Update contact for a user |
    | `DELETE` |**/users/{uid}/contacts/{id}**| Delete user contact |

## Design Concepts
### Probes
- Probes are events used to identify when a user has been contacted by the service to check on their aliveness.
- A probe can be in 5 possible states: 
    - `pending` - The server has sent out a probe message to the user & is awaiting a response e.g. `"Are you okay Stark?"`
    - `good` - The server has gotten a valid response for good i.e `Yes`, `Yeah`, `Yh` or `Y`
    - `bad` - The server has gotten a valid response for bad i.e `No`, `Nope`, `Nah` or `N`
    - `cancelled` - The probe was cancelled by the user via the rest API
    - `unavailable` - The server did not receive any response after multiple retries i.e `"You good ?"`
- In both a `bad` or `unavailable` state the server sends out a message to the user's emergency contact and then disables the probe.

## FAQ
- Q: Where's all the data stored ?
    - A: SQLite file

- Q: Why SQLite ?
    - A: For ease of use as you don't require a lot of external systems to set the kronus server up. Also the SQLite file is encrypted using the provided `passPhrase` with AES-256, see https://github.com/sqlcipher/sqlcipher.

- Q: Is this a dead man's switch ?
    - A: If you want it to be, sure. For now, it only sends out messsages to your emergency contacts if bad/no response is recieved by the server. Other extensions or use cases can be addded in the future.

- Q: Why ?
    - A: Why not ? It was/is a fun project to learn more `Go` and design/architecture patterns