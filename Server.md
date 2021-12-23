## Kronus `server` (pre-alpha)

The `server` is a new* feature currently in `pre-alpha`, and available through the `server` cmd on the CLI app.
```
kronus server -h
```
Kronus server allows you to schedule a liveliness probe for users on the server and sends out messages to emergency contacts if no/bad response is gotten from a user.

### Dependcies
- Google cloud storage credentials - (Optional - for db backup)
- Twilio account - for sending probe messages
- SQlite

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
    prefix: "kronus"
    
    # How often you want your sqlite db to be backed up to google storage in cron format
    sqliteBackupSchedule: "0 * * * *"

    enableSqliteBackupAndSync: false
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

### Usage
For now the only way to interact with the server resources is via the rest api:

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
    "password": "1234"
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

- `PUT` **/users/{uid}/probe_settings/** set to `true/false` to enable/disable probe
```json
{
    "active": true
}
```
- Admin only routes
    - `GET` **/users**
    - `GET` **/jobs/stats**
    - `GET` **/jobs?status=***
    - `GET` **/probes/stats**
    - `GET` **/probes?status=***
- Other routes
    - `POST` **/webhook/sms** - for twilio message webhook
    - `GET` **/jwks** - for validating kronus server jwts
    - `GET` **/health** - to check service health
    - `GET` **/users/{uid:[0-9]+}** - Can only GET your own record, except if you're admin
    - `PUT` **/users/{uid:[0-9]+}** - Can only UPDATE own record
    - `DELETE` **/users/{uid:[0-9]+}** - Can only DELETE your own record, except if you're admin
    - `GET` **/users/{uid:[0-9]+}/contacts**
    - `PUT` **/users/{uid:[0-9]+}/contacts/{id:[0-9]+}** - UPDATE contact
    - `DELETE` **/users/{uid:[0-9]+}/contacts/{id:[0-9]+}**

## Design Concepts

## FAQ
- Q: What is a liveliness probe ?
    - A: It's just a fancy way of saying the server `pings` it's humans to check that they are alive & doing okay. If the server determine's all's good, nothing happens.
    
    - However, if the server determines all's not good or doesn't receive any valid response after a ping, a message is sent to the user's emergency contact and the liveliness probe is turned off.

- Q: Why ?
    - A: Why not ?
