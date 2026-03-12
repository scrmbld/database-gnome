// Handles all the AI query generation logic
package gnome

import (
	"errors"
	"fmt"
	"strings"

	"github.com/scrmbld/database-gnome/cmd/glue"
)

type Gnome interface {
	// returns a SQL query
	Query(userRequest string) (string, error)
}

const model string = "openai/gpt-oss-20b"

const sqlSchema string = "```sql\\nCREATE TABLE IF NOT EXISTS \\\"Observation\\\" (\\n\\tmpg FLOAT,\\n\\tcylinders BIGINT,\\n\\tdisplacement FLOAT,\\n\\thorsepower FLOAT,\\n\\tweight BIGINT,\\n\\tacceleration FLOAT,\\n\\tmodel_year BIGINT,\\n\\torigin_id BIGINT,\\n\\tname_id BIGINT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Origin\\\" (\\n\\torigin_id BIGINT,\\n\\torigin TEXT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Name\\\" (\\n\\tname_id BIGINT,\\n\\tname TEXT\\n);\\n```\\nHere's an explanation of the meaning of each row:\\nObservation.mpg -- fuel economy in miles per gallon\\nObservation.cynlinders -- the cylinder count of the cars engine. For instance, a car with a V8 would would have an Observation.cynlinders of 8\\nObservation.horsepower -- the car's horsepower spec\\nObservation.wieght -- the car's weight in pounds\\nObservation.model_year -- the model year of the car\\nObservation.acceleration -- the car's 0 to 60 time\\nObservation.displacement -- the displacement of the engine in cubic inches\\nObservation.name_id -- foreign key for the name table.\\nObservation.origin_id -- foreign key for country of origin information\\nName.name_id -- primary key for Name table\\nName.name -- a vehicle name in the form \\\"make model\\\", not including model year. For example, \\\"Toyota Corolla\\\"\\nOrigin.origin_id -- primary key for Origin table\\nOrigin.origin -- country of origin, one of either \\\"japan\\\", \\\"europe\\\", or \\\"usa\\\"\\n"

const sqlTemplate string = "SELECT Name.name, Observation.mpg, Observation.cylinders, Observation.horsepower, Observation.weight, Observation.model_year, Observation.acceleration FROM Observation INNER JOIN Name ON Name.name_id=Observation.name_id INNER JOIN Origin on Origin.origin_id=Observation.origin_id WHERE"

// 1. ask for planned filters
const plannerPrompt string = "You are a part of a database agent system that enables users to filter products on an ecommerce website using natural language queries. Your task is to outline, in English, what conditions have to be put on the columns in order to fulfill the user's request. Make sure to avoid using SQL code in your answer, other than the column names themselvs. This plan will then be used by the query generator model to generate a SQL query. Here is the databse schema:\\n" + sqlSchema + "Once again, your task is to analyze the SQL schema and user query to plan out filters for the SQL generator to implement."

// 2. generate sql
const generatorPrompt string = "You are a part of a database agent system that enables users to filter products on an ecommerce website using natural language queries. Your task is to help generate sql queries based on the user input. Remember that, since you only help users filter, sort, and search product listings, you have no ability to perform any write operations to the database -- that includes DELETE, UPDATE, and INSERT operations. Also, you cannot influence the information shown on product listings, only which listings are shown and what order they are in. Here is the databse schema:\\n" + sqlSchema + "\\nThe system will run your SQL code to get a list of name_id values that match your filters. This list is then used to generate the web view. Your output must ONLY include SQL code in plain text format (no markdown). Anything else WILL break the system.\\n\\nThe planning stage has determined which rows are relevant to the user request, which you will receive with the user request as a comma separated list. When you receive a user request, complete the following SQL so that it returns the name_id of all products that match the said user request.\\n```sql\\n" + sqlTemplate + "```\\nDo not repeat the already provided SQL code in your response, only include the parts that you have come up with. Here is an example of an incorrectly formatted reponse:\\n`sql\\nSELECT Name.name, Observation.mpg, Observation.cylinders, Observation.horsepower, Observation.weight, Observation.model_year, Observation.acceleration FROM Observation INNER JOIN Name ON Name.name_id=Observation.name_id INNER JOIN Origin on Origin.origin_id=Observation.origin_id WHERE Observation.mpg > 20 `\\n This is incorrect because it is in markdown format, and includes the template. Here is an example of a correctly formatted reponse:\\nObservation.mpg > 20"

// 3. validate relatedness to database schema
const validatorPrompt string = "You are a part of a database agent system that enables users to filter products on an ecommerce website using natural language queries. Your task is to validate the generated SQL. Here's the schema for the SQLite database:" + sqlSchema + "\\nTrue or false, for the request and SQL query pairs determine, given this database schema, whether the SQL query answers the question. Your answer should only be either \\\"true\\\" or \\\"false\\\", without quotation marks, and should contain no other explanation. Keep in mind that the system completes the query starting from the template `" + sqlTemplate + "`, and the set of columns selected in the template are required by the user interface. Also, the query should not write to the database."

const maxRetries int = 3

const goFimSqlSchema string = `
// 	CREATE TABLE IF NOT EXISTS \"Observation\" (\n
// 	mpg FLOAT,\n
// 	cylinders BIGINT,\n
// 	displacement FLOAT,\n
// 	horsepower FLOAT,\n
// 	weight BIGINT,\n
// 	acceleration FLOAT,\n
// 	model_year BIGINT,\n
// 	origin_id BIGINT,\n
// 	name_id BIGINT\n
// );\n
// CREATE TABLE IF NOT EXISTS \"Origin\" (\n
// 	origin_id BIGINT,\n
// 	origin TEXT\n
// );\n
// CREATE TABLE IF NOT EXISTS \"Name\" (\n
// 	name_id BIGINT,\n
// 	name TEXT\n
// );`

type DefaultGnome struct {
	sqlTemplate string
	provider    glue.Provider
}

func (g *DefaultGnome) plan(userQuery string) (string, error) {
	response, err := g.provider.Request(model, plannerPrompt, userQuery)
	if err != nil {
		return "", err
	}
	// remove newlines and stuff to avoid JSON problems
	result := strings.ReplaceAll(response.Choices[0].Message.Content, "\n", "\\n")
	result = strings.ReplaceAll(result, "\t", "\\t")
	result = strings.ReplaceAll(result, "\"", "\\\"")
	return result, nil
}

func (g *DefaultGnome) generate(userQuery string, plan string) (string, error) {
	userPrompt := fmt.Sprintf("Original Query:\\n%s\\n\\nFormulated Plan:%s", userQuery, plan)
	response, err := g.provider.Request(model, generatorPrompt, userPrompt)
	if err != nil {
		return "", err
	}
	return response.Choices[0].Message.Content, nil
}

func (g *DefaultGnome) validateQuery(userQuery string, generatedFilters string) bool {
	// TODO: analysis of SQL code using treesitter or something
	sqlQuery := fmt.Sprintf("%s %s", sqlTemplate, generatedFilters)
	response, err := g.provider.Request(model, validatorPrompt, fmt.Sprintf("%s,\\n\\n%s", userQuery, sqlQuery))
	if err != nil {
		return false
	}
	responseText := response.Choices[0].Message.Content
	// strip whitespace, remove non alphabetical, check if true or fals
	responseText = strings.ToLower(strings.TrimSpace(responseText))
	return responseText == "true"
}

func (g *DefaultGnome) GenerateQuery(userQuery string) (string, error) {
	for i := 0; i < maxRetries; i++ {
		plan, err := g.plan(userQuery)
		if err != nil {
			return "", err
		}
		generatedSql, err := g.generate(userQuery, plan)
		if err != nil {
			return "", err
		}
		query := fmt.Sprintf("%s %s", g.sqlTemplate, generatedSql)

		if !g.validateQuery(userQuery, generatedSql) {
			continue
		}

		return query, nil
	}
	return "", errors.New("Could not generate a valid sql query")
}

func NewGnome(provider glue.Provider) DefaultGnome {
	result := DefaultGnome{
		sqlTemplate: sqlTemplate,
		provider:    provider,
	}

	return result
}
