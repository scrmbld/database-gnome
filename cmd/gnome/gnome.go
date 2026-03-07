// Handles all the AI query generation logic
package gnome

import (
	"encoding/json"
	"fmt"

	"github.com/scrmbld/database-gnome/cmd/glue"
)

type Gnome interface {
	// returns a SQL query
	Query(userRequest string) (string, error)
}

// 2. ask for filter conditions for each row (natural language)
// 3. ask for sorting condition (natural language)
// 4. use repeated FiM completion to generate SQL code for each condition

const groqSystemPrompt string = "You are a part of a database agent system that enables users to filter products on an ecommerce website using natural language queries. Your task is to help generate sql queries based on the user input. Remember that, since you only help users filter, sort, and search product listings, you have no ability to perform any write operations to the database -- that includes DELETE, UPDATE, and INSERT operations. Also, you cannot influence the information shown on product listings, only which listings are shown and what order they are in. Here is the databse schema:\\n```sql\\nCREATE TABLE IF NOT EXISTS \\\"Observation\\\" (\\n\\tmpg FLOAT,\\n\\tcylinders BIGINT,\\n\\tdisplacement FLOAT,\\n\\thorsepower FLOAT,\\n\\tweight BIGINT,\\n\\tacceleration FLOAT,\\n\\tmodel_year BIGINT,\\n\\torigin_id BIGINT,\\n\\tname_id BIGINT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Origin\\\" (\\n\\torigin_id BIGINT,\\n\\torigin TEXT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Name\\\" (\\n\\tname_id BIGINT,\\n\\tname TEXT\\n);\\n```\\nThe system will run your SQL code to get a list of name_id values that match your filters. This list is then used to generate the web view. Your output must ONLY include SQL code in plain text format (no markdown). Anything else WILL break the system.\\n\\nWhen you receive a user request, complete the following SQL so that it returns the name_id of all products that match the said user request.\\n```sql\\n" + groqSqlTemplate + "```\\nDo not repeat the already provided SQL code in your response, only include the parts that you have come up with."

const groqSqlTemplate string = "SELECT Name.name, Observation.mpg, Observation.cylinders, Observation.horsepower, Observation.weight, Observation.model_year, Observation.acceleration FROM Observation INNER JOIN Name ON Name.name_id=Observation.name_id INNER JOIN Origin on Origin.origin_id=Observation.origin_id WHERE"

// 1. ask which rows our filters will need
const groqPlannerTemplate string = "You are a part of a database agent system that enables users to filter products on an ecommerce website using natural language queries. Your task is to analyze the SQL schema to decide which rows are relevant to the user query. Here is the databse schema:\\n```sql\\nCREATE TABLE IF NOT EXISTS \\\"Observation\\\" (\\n\\tmpg FLOAT,\\n\\tcylinders BIGINT,\\n\\tdisplacement FLOAT,\\n\\thorsepower FLOAT,\\n\\tweight BIGINT,\\n\\tacceleration FLOAT,\\n\\tmodel_year BIGINT,\\n\\torigin_id BIGINT,\\n\\tname_id BIGINT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Origin\\\" (\\n\\torigin_id BIGINT,\\n\\torigin TEXT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Name\\\" (\\n\\tname_id BIGINT,\\n\\tname TEXT\\n);\\n```\\nHere's an explanation of the meaning of each row:\\nObservation.mpg -- fuel economy in miles per gallon\\nObservation.cynlinders -- the cylinder count of the cars engine. For instance, a car with a V8 would would have an Observation.cynlinders of 8\\nObservation.horsepower -- the car's horsepower spec\\nObservation.wieght -- the car's weight in pounds\\nObservation.model_year -- the model year of the car\\nObservation.acceleration -- the car's 0 to 60 time\\nObservation.displacement -- the displacement of the engine in cubic inches\\nObservation.name_id -- foreign key for the name table.\\nObservation.origin_id -- foreign key for country of origin information\\nName.name_id -- primary key for Name table\\nName.name -- a vehicle name in the form \"make model\", not including model year. For example, \"Toyota Corolla\"\\nOrigin.origin_id -- primary key for Origin table\\nOrigin.origin -- country of origin, one of either \"japan\", \"europe\", or \"usa\"\\nOnce again, your task is to analyze the SQL schema to decide which rows are relevant to the user query. Return your response in JSON format in the form `{\"columns\": [/* list of columns */]}`, with no other explanation or information. Here are some example outputs that follow this formatting:\\n\\n{\"columns\": [\"Observation.weight\", \"Observation.mpg\", \"Name.name\"]}\\n\\n{\"columns\": [\"Origin.origin\", \"Observation.cylinders\", \"Observation.horsepower\", \"Observation.acceleration\", \"Name.name_id\", \"Observation.displacement\"]}\\n\\n{\"columns\": [\"Observation.model_year\"]}\\n\\n"

type DefaultGnome struct {
	systemPrompt string
	sqlTemplate  string
	model        glue.Model
}

type planResponse struct {
	Columns []string `json:"columns"`
}

func (g *DefaultGnome) plan(userQuery string) ([]string, error) {
	response, err := g.model.Request(g.systemPrompt, userQuery)
	if err != nil {
		return nil, err
	}
	// parse the query
	var result planResponse
	err = json.Unmarshal([]byte(response.Choices[0].Message.Content), &result)
	if err != nil {
		return nil, err
	}
	return result.Columns, nil
}

func (g *DefaultGnome) GenerateQuery(userQuery string) (string, error) {
	// plannedRows, err := g.plan(userQuery)
	// if err != nil {
	// 	return "", err
	// }
	response, err := g.model.Request(g.systemPrompt, userQuery)
	if err != nil {
		return "", err
	}
	query := fmt.Sprintf("%s %s", g.sqlTemplate, response.Choices[0].Message.Content)
	return query, nil
}

func NewGnome(model glue.Model) DefaultGnome {
	result := DefaultGnome{
		systemPrompt: groqSystemPrompt,
		sqlTemplate:  groqSqlTemplate,
		model:        model,
	}

	return result
}
