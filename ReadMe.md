# Kronus
A CLI App to help you stay in touch with the people that matter by leveraging the [google calender API]().
You can create `touchbase` events for contacts in any group, up to a Max of **7** contacts.
```
The application is a tool to generate recurring google calender events for each of your contacts,
to remind you to reach out and see how they are doing :)

Usage:
  kronus [command]

Available Commands:
  completion  generate the autocompletion script for the specified shell
  help        Help about any command
  touchbase   Deletes all exisiting events and creates new ones based on configs

Flags:
      --config string   config file (default is $HOME/.kronus.yaml)
  -h, --help            help for kronus
  -t, --toggle          Help message for toggle
```

## How to use it
- Run `go get kronus` in terminal to install package
- In terminal run `kronus touchbase --group=family` to create re-curring event on google calendar
  - Supported touchbase flags include:
    ```
    Flags:
      -c, --count int          How many times you want to touchbase with members of a group (default 4)
      -f, --freq int           How often you want to touchbase i.e. 0 - weekly, 1 - bi-weekly, or 2 - monthly (default 1)
      -g, --group string       Group to create touchbase events for
      -h, --help               help for touchbase
      -t, --time-slot string   Time slot in the day allocated for touching base (default "18:00-18:30")
    ```

## Configuration
The config file is created in `$HOME/.kronus.yaml` if you've run the App at least once.

You can also create the config file manually in the default path if you choose. Or a new path & tell `kronus` where to find it using the `--config` flag.

Update config to include your `contacts` & `groups`. 
  ```yml
  env: production
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
  ```

## Development
- Checkout repo: 
  ```
  git clone https://github.com/Daskott/kronus.git
  ```
- In root dir from terminal run `go run main.go` for root cmd or `go run main.go touchbase --group=<your_group>` for touchbase cmd

## Todo:
* Write tests
* Add support for --version flag
* Ability to change event title
* Add flag to make deletion of previous events optional
* Support import of contacts from google csv
* Ability to set `client_id` & `client_secret` via CLI
* Ability to add/update contacts via CLI
* Ability to add/update contacts via CLI
* Ability to configure start date of touchbase events
* List events via CLI
