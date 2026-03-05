# Database Gnome

The Database Gnome is a simple database agent that performs SQL queries based on natural language user input.

## TODO

- [X] set up SQLite database for testing
    - modernc driver
- [X] create web view generator
- [X] connect to Groq API
- [] planning stage
    - decide on list of relevant columns
    - use this to generate template (?)
- [] generator stage
    - given query template, complete the query to filter based on user requests
- [] query validity checker
    - SQL injection (extraneous semicolons)
    - permissions errors
    - invalid SQL
- [] database permissions checker
- [] query executor
- [] connect to llama.cpp to experiment with smaller models
