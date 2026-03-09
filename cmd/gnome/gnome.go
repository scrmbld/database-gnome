// Handles all the AI query generation logic
package gnome

import (
	"fmt"
	"strings"

	"github.com/scrmbld/database-gnome/cmd/glue"
)

type Gnome interface {
	// returns a SQL query
	Query(userRequest string) (string, error)
}

// 2. ask for filter conditions for each row (natural language)
// 3. ask for sorting condition (natural language)
// 4. use repeated FiM completion to generate SQL code for each condition

const systemPrompt string = "You are a part of a database agent system that enables users to filter products on an ecommerce website using natural language queries. Your task is to help generate sql queries based on the user input. Remember that, since you only help users filter, sort, and search product listings, you have no ability to perform any write operations to the database -- that includes DELETE, UPDATE, and INSERT operations. Also, you cannot influence the information shown on product listings, only which listings are shown and what order they are in. Here is the databse schema:\\n```sql\\nCREATE TABLE IF NOT EXISTS \\\"Observation\\\" (\\n\\tmpg FLOAT,\\n\\tcylinders BIGINT,\\n\\tdisplacement FLOAT,\\n\\thorsepower FLOAT,\\n\\tweight BIGINT,\\n\\tacceleration FLOAT,\\n\\tmodel_year BIGINT,\\n\\torigin_id BIGINT,\\n\\tname_id BIGINT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Origin\\\" (\\n\\torigin_id BIGINT,\\n\\torigin TEXT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Name\\\" (\\n\\tname_id BIGINT,\\n\\tname TEXT\\n);\\n```\\nThe system will run your SQL code to get a list of name_id values that match your filters. This list is then used to generate the web view. Your output must ONLY include SQL code in plain text format (no markdown). Anything else WILL break the system.\\n\\nThe planning stage has determined which rows are relevant to the user request, which you will receive with the user request as a comma separated list. When you receive a user request, complete the following SQL so that it returns the name_id of all products that match the said user request.\\n```sql\\n" + sqlTemplate + "```\\nDo not repeat the already provided SQL code in your response, only include the parts that you have come up with. Here is an example of an incorrectly formatted reponse:\\n`sql\\nSELECT Name.name, Observation.mpg, Observation.cylinders, Observation.horsepower, Observation.weight, Observation.model_year, Observation.acceleration FROM Observation INNER JOIN Name ON Name.name_id=Observation.name_id INNER JOIN Origin on Origin.origin_id=Observation.origin_id WHERE Observation.mpg > 20 `\\n This is incorrect because it is in markdown format, and includes the template. Here is an example of a correctly formatted reponse:\\nObservation.mpg > 20"

const sqlTemplate string = "SELECT Name.name, Observation.mpg, Observation.cylinders, Observation.horsepower, Observation.weight, Observation.model_year, Observation.acceleration FROM Observation INNER JOIN Name ON Name.name_id=Observation.name_id INNER JOIN Origin on Origin.origin_id=Observation.origin_id WHERE"

// 1. ask which rows our filters will need
const rowSelectorTemplate string = "You are a part of a database agent system that enables users to filter products on an ecommerce website using natural language queries. Your task is to analyze the database and plan out what filters are needed to fulfill the user's request. This plan will then be used by the query generator model to generate a SQL query. Write your plan as though you are speaking to a programmer; don't be afraid to use technical language. Here is the databse schema:\\n```sql\\nCREATE TABLE IF NOT EXISTS \\\"Observation\\\" (\\n\\tmpg FLOAT,\\n\\tcylinders BIGINT,\\n\\tdisplacement FLOAT,\\n\\thorsepower FLOAT,\\n\\tweight BIGINT,\\n\\tacceleration FLOAT,\\n\\tmodel_year BIGINT,\\n\\torigin_id BIGINT,\\n\\tname_id BIGINT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Origin\\\" (\\n\\torigin_id BIGINT,\\n\\torigin TEXT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Name\\\" (\\n\\tname_id BIGINT,\\n\\tname TEXT\\n);\\n```\\nHere's an explanation of the meaning of each row:\\nObservation.mpg -- fuel economy in miles per gallon\\nObservation.cynlinders -- the cylinder count of the cars engine. For instance, a car with a V8 would would have an Observation.cynlinders of 8\\nObservation.horsepower -- the car's horsepower spec\\nObservation.wieght -- the car's weight in pounds\\nObservation.model_year -- the model year of the car\\nObservation.acceleration -- the car's 0 to 60 time\\nObservation.displacement -- the displacement of the engine in cubic inches\\nObservation.name_id -- foreign key for the name table.\\nObservation.origin_id -- foreign key for country of origin information\\nName.name_id -- primary key for Name table\\nName.name -- a vehicle name in the form \\\"make model\\\", not including model year. For example, \\\"Toyota Corolla\\\"\\nOrigin.origin_id -- primary key for Origin table\\nOrigin.origin -- country of origin, one of either \\\"japan\\\", \\\"europe\\\", or \\\"usa\\\"\\nOnce again, your task is to analyze the SQL schema and user query to plan out filters for the SQL generator to implement."

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

const fimBeforeTemplate string = "// Database Schema:\\n" + goFimSqlSchema
const fimAfterTemplate string = "ORDER BY"

type DefaultGnome struct {
	sqlTemplate string
	provider    glue.Provider
}

func (g *DefaultGnome) plan(userQuery string) (string, error) {
	response, err := g.provider.Request("smollm", rowSelectorTemplate, userQuery)
	if err != nil {
		return "", err
	}
	return response.Choices[0].Message.Content, nil
}

func (g *DefaultGnome) generate(userQuery string, plan string) (string, error) {
	fimBefore := fmt.Sprintf("%s\\n/*\\nUser requirements: \\\"%s\\\"\\n*/\\n/*\\nTODO: %s\\n*/\\nsqlQuery := \\\"%s", fimBeforeTemplate, userQuery, plan, sqlTemplate)
	fimAfter := fimAfterTemplate
	flatBefore := strings.ReplaceAll(strings.ReplaceAll(fimBefore, "\n", "\\n"), "\t", "\\t")
	flatAfter := strings.ReplaceAll(strings.ReplaceAll(fimAfter, "\n", "\\n"), "\t", "\\t")
	response, err := g.provider.FimRequest("starcoder2", flatBefore, flatAfter)
	if err != nil {
		return "", err
	}
	return response.Content, nil
}

func (g *DefaultGnome) GenerateQuery(userQuery string) (string, error) {
	plan, err := g.plan(userQuery)
	if err != nil {
		return "", err
	}
	generatedSql, err := g.generate(userQuery, plan)
	if err != nil {
		return "", err
	}
	query := fmt.Sprintf("%s %s", g.sqlTemplate, generatedSql)
	return query, nil
}

func NewGnome(provider glue.Provider) DefaultGnome {
	result := DefaultGnome{
		sqlTemplate: sqlTemplate,
		provider:    provider,
	}

	return result
}
