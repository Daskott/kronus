## Kronus
Kronus is a tool for asessing users "aliveness" via liveliness probes and
actioning on it based on the results of the probe.

Its key features are:
- **Sending Liveliness Probes:** The kronus server sends out probes to users on the
  server and based on their response, determines their "aliveness".

- **Contact Emergency Contact:** When the service doesn't get a response back
  from a user or gets a `bad` response, the user's emergency contact is alerted.

- **Robust API:** Allow developers to add extra functionality based on a user's
  "aliveness". A user's `probes` can be queried periodically from kronus server
  to see the `status` of the last probe. And based on that, do whatever the
  developer wants.

```
Usage:
  kronus [command]

Available Commands:
  completion  generate the autocompletion script for the specified shell
  help        Help about any command
  server      Start a kronus server

Flags:
      --dev       run in development mode
  -h, --help      help for kronus
      --test      run in test mode
  -t, --toggle    Help message for toggle
  -v, --version   version for kronus

Use "kronus [command] --help" for more information about a command.
  ```

### Dependencies
- Install [Go](https://golang.org/dl/)
- [Twilio](https://www.twilio.com/) sms webhook(use POST `/webhook/sms` endpoint) and credentials for
  sending probe messages.
- [Google application credentials](https://cloud.google.com/iam/docs/creating-managing-service-accounts#iam-service-accounts-create-console) for [cloud storage](https://cloud.google.com/storage) - (Optional - for sqlite file backup).

### Install
```
go get -u github.com/Daskott/kronus
```

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
    port: 3900

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
- The first user created is assigned the `admin` role and every other user has to be created by the `admin`.
  <br/>A default `probe_settings` is created for the user account, and `active`
  is set to `false`.
  
  | Method | Path |
  | --- | --- |
  | `POST` | **/webhook/sms** |
    
  <br/>**Sample Request:**
  ```curl
  curl --request POST 'localhost:3900/v1/users' \
  --header 'Authorization: Bearer <token>' \
  --header 'Content-Type: application/json' \
  --data-raw '{
      "first_name": "tony",
      "last_name": "stark",
      "email": "stark@avengers.com",
      "password": "very-secure",
      "phone_number": "+12345678900"
  }'
  ```
  <br/>**Sample Response:**
  ```json
    {
      "success": true,
      "data": {
          "id": 1,
          "created_at": "2022-01-10T19:54:53.708959-07:00",
          "updated_at": "2022-01-10T19:54:53.708959-07:00",
          "first_name": "tony",
          "last_name": "stark",
          "phone_number": "+12345678900",
          "email": "stark@avengers.com",
          "role_id": 1,
          "probe_settings": {
              "id": 1,
              "created_at": "2022-01-10T19:54:53.709185-07:00",
              "updated_at": "2022-01-10T19:54:53.709185-07:00",
              "user_id": 1,
              "active": false,
              "cron_expression": "0 18 * * 3"
          }
      }
  }
  ```

#### Get access token
- Get access `token` which will be used to query protected resources 
  
  | Method | Path |
  | --- | --- |
  | `POST` | **/login** |

  <br/>**Sample Request:**
  ```curl
  curl --request POST 'localhost:3900/login' \
  --header 'Content-Type: application/json' \
  --data-raw '{
      "email": "stark@avengers.com",
      "password": "very-secure"
  }'
  ```
  <br/>**Sample Response:**
  ```json
    {
      "success": true,
      "data": {
          "token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...."
      }
    }
  ```

#### Create contact
-  For protected routes, the `token` from the **/login** needs to be added to the `Authorization` header as `Bearer <token>`
  
    | Method | Path |
    | --- | --- |
    | `POST` | **/users/{uid}/contacts/** |

    <br/>**Sample Request:**
    ```curl
    curl --location --request POST 'localhost:3900/v1/users/1/contacts' \
    --header 'Authorization: Bearer <token>' \
    --header 'Content-Type: application/json' \
    --data-raw '{
        "first_name": "strongest",
        "last_name": "avenger",
        "phone_number": "+12345678900",
        "email": "hulk@avengers.com",
        "is_emergency_contact": true
    }'
    ```
    <br/>**Sample Response:**
    ```json
    {
      "success": true,
      "data": {
          "id": 1,
          "created_at": "2022-01-10T20:25:40.878785-07:00",
          "updated_at": "2022-01-10T20:25:40.878785-07:00",
          "first_name": "strongest",
          "last_name": "avenger",
          "phone_number": "+12345678900",
          "email": "hulk@avengers.com",
          "user_id": 1,
          "is_emergency_contact": true
      }
    }
    ```   

#### Update probe settings
- Set how often you'd like to get a probe message with a `cron_expression` and use `active` to
  enable/disable probe.

  | Method | Path |
  | --- | --- |
  | `PUT` | **/users/{uid}/probe_settings/** |

  <br/>**Sample Request:**
  ```curl
  curl --request PUT 'localhost:3900/v1/users/1/probe_settings' \
  --header 'Authorization: Bearer <token>' \
  --data-raw '{
      "cron_expression": "0 18 * * */1",
      "active": true
  }'
  ```
  <br/>**Sample Response:**
  ```json
    {
      "success": true,
      "data": {
          "id": 1,
          "created_at": "2022-01-10T19:54:53.709185-07:00",
          "updated_at": "2022-01-10T20:33:16.828639-07:00",
          "user_id": 1,
          "active": true,
          "cron_expression": "0 18 * * */1"
      }
  }
  ```

#### Other routes

| Method | Route | Note |
| --- | --- | --- |
| `POST` | **/webhook/sms** | For twilio message webhook |
| `GET` | **/jwks** | For validating kronus server jwts |
| `GET` | **/health** | To check service health |
| `GET` | **/v1/users/{uid}**| Can only GET your own record, except if you're admin |
| `PUT` |**/v1/users/{uid}**| Can only UPDATE own record |
| `DELETE` |**/v1/users/{uid}**| Can only DELETE your own record, except if you're admin |
| `GET` |**/v1/users/{uid}/contacts**| Fetch all contacts for a given user where `uid` is the user id. Supports optional `page` filter for pagination|
| `GET` |**/v1/users/{uid}/probes**| Fetch all probes for a given user where `uid` is the user id. Supports optional `page` filter for pagination |
| `PUT` |**/v1/users/{uid}/contacts/{id}**| Update contact for a user |
| `DELETE` |**/v1/users/{uid}/contacts/{id}**| Delete user contact |
| `GET` | **/v1/users** | Fetch all users. Supports optional `page` filter for pagination ***[admin-only]*** |
| `GET` | **/v1/jobs/stats** | Get job stats i.e. no of jobs in each group e.g. `enqueued`, `successful`, `in-progress` or `dead` - ***[admin-only]***|
| `GET` | **/v1/jobs?status=** | Fetch jobs with optional filter - *status* which could be `enqueued`, `successful`, `in-progress` or `dead`. Also supports pagination - ***[admin-only]***|
| `GET` | **/v1/probes/stats** | Get probe stats i.e. no of probes in each group e.g. `pending`, `good`, `bad` `cancelled`, or `unavailable` - ***[admin-only]***|
| `GET` | **/v1/probes?status=** | Fetch probes with optional filter - *status* which could be  `pending`, `good`, `bad` `cancelled`, or `unavailable`. Also supports pagination - ***[admin-only]***|

### Development
- Checkout repo: 
  ```
  git clone https://github.com/Daskott/kronus.git
  ```
- To run `rootCmd`: 
  ```
  go run main.go
  ```
- To run `server`: 
  ```
  go run main.go server --dev
  ```
- To run tests: 
  ```
  make test
  ```

### Publishing package
- Update `Version` in `version.go`

- Commit changes:
  ```
  git commit -m "kronus: changes for v0.3.2"
  ```

- Run:
  ```
  make release VERSION=0.3.2
  ```
For more info see detailed steps https://golang.org/doc/modules/publishing

## Design Concepts
### Probes
- Probes are events used to identify when a user has been contacted by the service to check on their aliveness.
- A probe can be in 5 possible states: 
    - `pending` - The server has sent out a probe message to the user & is awaiting a response e.g. `"Are you okay Stark?"`
    - `good` - The server has gotten a valid response for good i.e `Yes`, `Yeah`, `Yh` or `Y`
    - `bad` - The server has gotten a valid response for bad i.e `No`, `Nope`, `Nah` or `N`
    - `cancelled` - The probe was cancelled by the user via the rest API
    - `unavailable` - The server did not receive any response after multiple retries i.e `"You good ??"`
- In both a `bad` or `unavailable` state the server sends out a message to the
  user's emergency contact and then disables the probe.

## FAQ
- Q: Does this work in a distributed environment ?
    - A: No. This doesn't work in a distributed environment at the moment, because the server uses Sqlite3 to store all it's data.
         As a result, if you deploy this to a production environment, it should run on a single pod/machine.

- Q: Why SQLite ?
    - A1: I don't want to pay for a hosted database 😅
    - A2: For ease of use, as you don't require a lot of external systems to get started. For data protection, the SQLite file is encrypted using the provided `passPhrase` with AES-256, see https://github.com/sqlcipher/sqlcipher.

- Q: Is this a dead man's switch ?
    - A: If you want it to be, sure. For now, it only sends out messsages to your emergency contacts if bad/no response is recieved by the server. Other extensions or use cases can be addded in the future.

- Q: Why ?
    - A: Why not ? Its a fun project to level up on `Go`, and design/architecture patterns.
