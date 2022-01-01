## Kronus `server` (pre-alpha)

The server schedules a liveliness probe for users in the database and sends out messages to emergency contacts if no/bad response is gotten from a user for a probe.

### Dependencies
- [Google application credentials](https://cloud.google.com/iam/docs/creating-managing-service-accounts#iam-service-accounts-create-console) for [cloud storage](https://cloud.google.com/storage) - (Optional - for sqlite file backup)
- [Twilio account](https://www.twilio.com/) credentials - for sending probe messages

### Server Config
The server requires a valid `config.yml` configuration file as shown below:
```yml
kronus:
  # A valid RSA private key for creating/validating kronus server jwts
  # You can can use this https://mkjwk.org/ to generate one
  privateKeyPem:

  # If deployed to a production env, the public url of the server.
  # It's required, as this will be used for the twilio webhook 
  publicUrl: "https://my-app.com"
  
  cron:
    # Timezone to use for scheduling probes
    timeZone: "America/Toronto"
  
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
  
  # The path to google service account credentials. You may not need to set this, if the
  # kronus server is deployed to a google compute engine instance - the default credentials may be used instead.
  applicationCredentials: "/path/to/google/credentials"

twilio:
  accountSid: AHKX00SXXXXXXXXXXXXXXXXXXX
  authToken: BHKX00SXXXXXXXXXXXXXXXXXXX
  messagingServiceSid: CHKX00SXXXXXXXXXXXXXXXXXXX
```

### Start server in dev mode
```
kronus server --dev
```

### Start server with config
```
kronus server --config=config.yml
```

### Setup steps
- [Create a user](#create-user) account
- [Get access token](#get-access-token) for protected routes
- [Add a contact](#create-contact) and set as user's emergency contact
- Finally [turn on the emergency probe](#update-probe-settings)


### API and Usage

#### Create user
- `POST` **/users** <br/>The first user created is assigned the `admin` role and every other user has to be created by the `admin`.
  <br/>A default `probe_settings` is created for the user account, and `active` is set to `false`.
  ```json
  {
      "first_name": "tony",
      "last_name": "stark",
      "email": "stark@avengers.com",
      "password": "very-secure",
      "phone_number": "+12345678900"
  }
  ```

#### Get access token
- `POST` **/login** <br/> Get access `token` which will be used to query protected resources 
  ```json
  {
      "email": "stark@avengers.com",
      "password": "very-secure"
  }
  ```

#### Create contact
- `POST` **/users/{uid}/contacts/** <br/> For protected routes, the `token` from the **/login** needs to be added to the `Authorization` header as `Bearer <token>`
  ```json
  {
      "first_name": "strongest",
      "last_name": "avenger",
      "phone_number": "+12345678900",
      "email": "hulk@avengers.com",
      "is_emergency_contact": true
  }
  ```

#### Update probe settings
- `PUT` **/users/{uid}/probe_settings/** <br/> Set how often you'd like to get a probe message with a `cron_expression` and use `active` to enable/disable probe.
  ```json
  {
      "cron_expression": "0 18 * * */1",
      "active": true
  }
  ```

#### Other routes

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
| `GET` | **/users** | Fetch all users ***[admin-only]*** |
| `GET` | **/jobs/stats** | Get job stats i.e. no of jobs in each group e.g. `enqueued`, `successful`, `in-progress` or `dead` - ***[admin-only]***|
| `GET` | **/jobs?status=** | Fetch jobs by status where status could be `enqueued`, `successful`, `in-progress` or `dead` - ***[admin-only]***|
| `GET` | **/probes/stats** | Get job stats i.e. no of probes in each group e.g. `pending`, `good`, `bad` `cancelled`, or `unavailable` - ***[admin-only]***|
| `GET` | **/probes?status=** | Fetch probes by status where status could be  `pending`, `good`, `bad` `cancelled`, or `unavailable` - ***[admin-only]***|

## Design Concepts
### Probes
- Probes are events used to identify when a user has been contacted by the service to check on their aliveness.
- A probe can be in 5 possible states: 
    - `pending` - The server has sent out a probe message to the user & is awaiting a response e.g. `"Are you okay Stark?"`
    - `good` - The server has gotten a valid response for good i.e `Yes`, `Yeah`, `Yh` or `Y`
    - `bad` - The server has gotten a valid response for bad i.e `No`, `Nope`, `Nah` or `N`
    - `cancelled` - The probe was cancelled by the user via the rest API
    - `unavailable` - The server did not receive any response after multiple retries i.e `"You good ??"`
- In both a `bad` or `unavailable` state the server sends out a message to the user's emergency contact and then disables the probe.

## FAQ
- Q: Does this work in a distributed environment ?
    - A: No. This doesn't work in a distributed environment at the moment, because the server uses Sqlite3 to store all it's data.
         As a result, if you deploy this to a production environment, it should run on a single pod/machine.

- Q: Why SQLite ?
    - A1: I don't want to pay for a hosted database
    - A2: For ease of use, as you don't require a lot of external systems to get started. For data protection, the SQLite file is encrypted using the provided `passPhrase` with AES-256, see https://github.com/sqlcipher/sqlcipher.

- Q: Is this a dead man's switch ?
    - A: If you want it to be, sure. For now, it only sends out messsages to your emergency contacts if bad/no response is recieved by the server. Other extensions or use cases can be addded in the future.

- Q: Why ?
    - A: Why not ? It was/is a fun project to learn more `Go` and design/architecture patterns
