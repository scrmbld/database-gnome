# Database Gnome

The Database Gnome is a simple database agent that performs SQL queries based on natural language user input. It requires either a groq API key or a local llama-server in order to run. Currently, the provider selection is hardcoded, so selecting one or the other requires changing the soruce code.

## Usage

1. clone or download the repository
2. build the docker container: `docker build -t database-gnome`
4. run `docker run --env GROQ_API_KEY=<your API key> -p 4400:4400 database-gnome` -- the web server should now be running
5. navigate to `localhost:4400` in your browser to see the site

At this point, you should be able to use the agent. The SQLite database comes included with the repo.

## Examples

- TODO: take screenshots

## Architecture

The system uses the Go standard library for the HTTP server and html templates. Server side code can be found in cmd, html templates in views, and static content like stylesheets can be found in static. The server side code is split into main, which starts the HTTP server and sets up the routes, glue, which contains code for accessing LLM provider APIs, and gnome, which implements the actual agent itself. Prompt templates can be found as consts in `gnome.go`:

```go
const sqlSchema string = "```sql\\nCREATE TABLE IF NOT EXISTS \\\"Observation\\\" (\\n\\tmpg FLOAT,\\n\\tcylinders BIGINT,\\n\\tdisplacement FLOAT,\\n\\thorsepower FLOAT,\\n\\tweight BIGINT,\\n\\tacceleration FLOAT,\\n\\tmodel_year BIGINT,\\n\\torigin_id BIGINT,\\n\\tname_id BIGINT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Origin\\\" (\\n\\torigin_id BIGINT,\\n\\torigin TEXT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Name\\\" (\\n\\tname_id BIGINT,\\n\\tname TEXT\\n);\\n```\\nHere's an explanation of the meaning of each row:\\nObservation.mpg -- fuel economy in miles per gallon\\nObservation.cynlinders -- the cylinder count of the cars engine. For instance, a car with a V8 would would have an Observation.cynlinders of 8\\nObservation.horsepower -- the car's horsepower spec\\nObservation.wieght -- the car's weight in pounds\\nObservation.model_year -- the model year of the car\\nObservation.acceleration -- the car's 0 to 60 time\\nObservation.displacement -- the displacement of the engine in cubic inches\\nObservation.name_id -- foreign key for the name table.\\nObservation.origin_id -- foreign key for country of origin information\\nName.name_id -- primary key for Name table\\nName.name -- a vehicle name in the form \\\"make model\\\", not including model year. For example, \\\"Toyota Corolla\\\"\\nOrigin.origin_id -- primary key for Origin table\\nOrigin.origin -- country of origin, one of either \\\"japan\\\", \\\"europe\\\", or \\\"usa\\\"\\n"

const sqlTemplate string = "SELECT Name.name, Observation.mpg, Observation.cylinders, Observation.horsepower, Observation.weight, Observation.model_year, Observation.acceleration FROM Observation INNER JOIN Name ON Name.name_id=Observation.name_id INNER JOIN Origin on Origin.origin_id=Observation.origin_id WHERE"

// 1. ask for planned filters
const plannerPrompt string = "You are a part of a database agent system that enables users to filter products on an ecommerce website using natural language queries. Your task is to outline, in English, what conditions have to be put on the columns in order to fulfill the user's request. Make sure to avoid using SQL code in your answer, other than the column names themselvs. This plan will then be used by the query generator model to generate a SQL query. Here is the databse schema:\\n" + sqlSchema + "Once again, your task is to analyze the SQL schema and user query to plan out filters for the SQL generator to implement."

// 2. generate sql
const generatorPrompt string = "You are a part of a database agent system that enables users to filter products on an ecommerce website using natural language queries. Your task is to help generate sql queries based on the user input. Remember that, since you only help users filter, sort, and search product listings, you have no ability to perform any write operations to the database -- that includes DELETE, UPDATE, and INSERT operations. Also, you cannot influence the information shown on product listings, only which listings are shown and what order they are in. Here is the databse schema:\\n" + sqlSchema + "\\nThe system will run your SQL code to get a list of name_id values that match your filters. This list is then used to generate the web view. Your output must ONLY include SQL code in plain text format (no markdown). Anything else WILL break the system.\\n\\nThe planning stage has determined which rows are relevant to the user request, which you will receive with the user request as a comma separated list. When you receive a user request, complete the following SQL so that it returns the name_id of all products that match the said user request.\\n```sql\\n" + sqlTemplate + "```\\nDo not repeat the already provided SQL code in your response, only include the parts that you have come up with. Here is an example of an incorrectly formatted reponse:\\n`sql\\nSELECT Name.name, Observation.mpg, Observation.cylinders, Observation.horsepower, Observation.weight, Observation.model_year, Observation.acceleration FROM Observation INNER JOIN Name ON Name.name_id=Observation.name_id INNER JOIN Origin on Origin.origin_id=Observation.origin_id WHERE Observation.mpg > 20 `\\n This is incorrect because it is in markdown format, and includes the template. Here is an example of a correctly formatted reponse:\\nObservation.mpg > 20"

// 3. validate relatedness to database schema
const validatorPrompt string = "Here's the schema for a SQL database:" + sqlSchema + "\\nTrue or false, for the request and SQL query pairs determine, given this database schema, whether the SQL query will answer the question. Your answer should only be either \\\"true\\\" or \\\"false\\\", without quotation marks, and should contain no other explanation."
```

The agent architecture is a relatively simplistic planner, generator, and validator, with one LLM call for each. There are currently has no tool calls and minimal enforced structure in how outputs are passed between phases. I experimented with more restrictive structures, including generating and parsing lists of columns relevant to a user query and using infill completion to generate SQL filters one at a time, but I found that model selection and tweaking system prompts had a much larger impact on performance.

Due to the structure of the application and the tools available in the Go ecosystem, I wasn't able to find a solution for query generation flexible enough for my application other than using raw SQL. This makes query validation difficult. I don't know of a solution that doesn't involve parsing the SQL code. My current plan is to use the Treesitter Go bindings, but I'm not sure this will give me all the information I need for my validation. The only alternative that I am aware of is writing my own parser.

## "Testing"

Running `test.py` while the webserver is active will run the database agent on a bunch of user queries, and then check that the agent's response matches a predetermined correct output. This does not test the system against prompt injection attacks as of yet. The test cases can be seen in `test_questions.csv`

## TODO

- [X] set up SQLite database for testing
    - modernc driver
- [x] create web view generator
- [x] connect to Groq API
- [x] planning stage
- [x] generator stage
- [ ] database permissions checker
- [ ] SQL injection checker
- [ ] SQL syntax validity checker
- [x] SQL query LLM analysis checker
- [x] query executor
- [x] connect to llama.cpp to experiment with smaller models
- [ ] make it not hideous
- [x] docker
