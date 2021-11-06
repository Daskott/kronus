# Kronus
A CLI App to help you stay in touch with the people that matter by leveraging the [google calender API](https://developers.google.com/calendar/api/guides/overview).
You can create `touchbase` events for contacts in any group, up to a Max of **7** contacts.
```
  kronus is a CLI library for Go that allows you to create
  coffee chat appointments with your contacts.

  The application is a tool to generate recurring google calender events for each of your contacts,
  to remind you to reach out and see how they are doing :)

  Usage:
    kronus [command]

  Available Commands:
    completion  generate the autocompletion script for the specified shell
    help        Help about any command
    touchbase   Deletes previous touchbase events and creates new ones based on configs

  Flags:
        --config string   config file (default is $HOME/.kronus.yaml)
    -h, --help            help for kronus
    -t, --toggle          Help message for toggle
    -v, --version         version for kronus

  Use "kronus [command] --help" for more information about a command.
  ```

## Pre-requisite
- Install [Go](https://golang.org/dl/)
- Create a google cloud project with permission to access google calendar. [See docs](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
- Create a service account to use with app. [See docs](https://cloud.google.com/iam/docs/creating-managing-service-accounts)
- Share your calendar with the service account email. Change the permission settings to `'Make changes to events'`. Save Changes.
- Make sure `GOOGLE_APPLICATION_CREDENTIALS` is set in the environment you run `kronus`(or update `$HOME/.kronus.yaml` accordingly - see below). E.g:
  ```
  export GOOGLE_APPLICATION_CREDENTIALS="/home/user/Downloads/service-account-file.json"
  ```

## How to use it
- Install:
  ```
  go get -u github.com/Daskott/kronus
  ```
- Run `kronus touchbase --group=family` to create re-curring event on google calendar
- For help run `kronus touchbase --help`
  ```
  Deletes previous touchbase google calender events created by kronus
        and creates new ones(up to a max of 7 contacts for a group) to match the values set in .kronus.yaml

  Usage:
    kronus touchbase [flags]

  Flags:
    -c, --count int          how many times you want to touchbase with members of a group (default 4)
    -d, --dev                run touchbase in development mode
    -f, --freq int           how often you want to touchbase i.e. 0 - weekly, 1 - bi-weekly, or 2 - monthly (default 1)
    -g, --group string       group to create touchbase events for
    -h, --help               help for touchbase
        --test               run touchbase in test mode
    -t, --time-slot string   time slot in the day allocated for touching base (default "18:00-18:30")

  Global Flags:
        --config string   config file (default is $HOME/.kronus.yaml)
  ```

## Configuration
The config file is created in `$HOME/.kronus.yaml` if you've run the App at least once.

You can also create the config file manually in the default path if you choose. Or a new path & tell `kronus` where to find it using the `--config` flag.

Update config to include your `timezone`, `contacts` & `groups`. 
  ```yml
  settings:
    # Update the timezone to match yours e.g.
    # America/New_York
    # America/Vancouver
    # America/Los_Angeles
    # Go to http://www.timezoneconverter.com/cgi-bin/findzone to see others.
    timezone: "America/Toronto"
    
    # Leave as is, to avoid unexpected behaviour. 
    touchbase-recurrence: "RRULE:FREQ=WEEKLY;"

  # Here you update your contact list with their names.
  # e.g.
  contacts:
    - name: Smally
    - name: Dad

  # Here you add the different groups you'd like to have for your
  # contacts. And populate each group with 
  # each contact's id(i.e. index of their record in contacts)
  # e.g. 
  groups:
    friends:
      - 0
      - 1
    family:
      - 0

  # This section is automatically updated by the CLI App to manage
  # events created by kronus
  events:

  owner:
    email: <The email associated with your google calendar>
  
  # For API secrets. This is mostly for convienece. In a production environment, pass GOOGLE_APPLICATION_CREDENTIALS directly into the env and kronus will override whatever is in here.
  secrets:
    GOOGLE_APPLICATION_CREDENTIALS: <Path to the JSON file that contains your service account key>
  ```

## Development
- Checkout repo: 
  ```
  git clone https://github.com/Daskott/kronus.git
  ```
- To run `rootCmd`: 
  ```
  go run main.go
  ```
- To run `touchbaseCmd`: 
  ```
  go run main.go touchbase
  ```
- To run tests: 
  ```
  make test
  ```

## Publishing package
* Update `Version` in `version.go`
* Commit changes:
  ```
  git commit -m "kronus: changes for v0.1.0"
  ```
* Run:
  ```
  make release VERSION=0.1.0
  ```
For more info see detailed steps https://golang.org/doc/modules/publishing
