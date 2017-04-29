package api_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/lucapette/deloominator/pkg/api"
	"github.com/lucapette/deloominator/pkg/db"
	"github.com/lucapette/deloominator/pkg/testutil"
)

var update = flag.Bool("update", false, "update golden files")

type test struct {
	query   string
	code    int
	fixture string
}

func graphqlPayload(t *testing.T, query string) string {
	payload := struct {
		Query string `json:"query"`
	}{Query: query}

	json, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf(err.Error())
	}

	return string(json)
}

func TestGraphQLQueries(t *testing.T) {
	dsn, cleanup := testutil.SetupPG(t)
	cfg := testutil.InitConfig(t, map[string]string{
		"DATA_SOURCES": dsn,
	})
	dataSources, err := db.NewDataSources(cfg.Sources)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		dataSources.Shutdown()
		cleanup()
	}()

	rows := db.QueryResult{
		Rows:    []db.Row{{db.Cell{Value: "42"}, db.Cell{Value: "Anna"}, db.Cell{Value: "Torv"}}},
		Columns: []db.Column{{Name: "actor_id"}, {Name: "first_name"}, {Name: "last_name"}},
	}

	for _, dataSource := range dataSources {
		testutil.LoadData(t, dataSource, "actor", rows)

		tests := []test{
			{query: "{ notAQuery }", code: 400, fixture: "wrong_query.json"},
			{query: "{ dataSources {name} }", code: 200, fixture: "data_sources.json"},
			{query: "{ dataSources {name tables {name}}}", code: 200, fixture: "data_sources_with_tables.json"},
			{
				query: fmt.Sprintf(`{ query(source: "%s", input: "select actor_id, first_name, last_name from actor") {
			                                          ... on results {
														chartName
									                    columns { name type }
									                    rows { cells { value } }
								                      }
		                                            }
	                                              }`, dataSource.Name()),
				code:    200,
				fixture: "query_raw_results.json",
			},
			{
				query: fmt.Sprintf(`{ query(source: "%s", input: "select substr(first_name, 1, 1) initial, count(*)  from actor group by 1") {
			                                          ... on results {
														chartName
									                    columns { name type }
									                    rows { cells { value } }
								                      }
		                                            }
	                                              }`, dataSource.Name()),
				code:    200,
				fixture: "query_chart_detected.json",
			},
			{
				query: fmt.Sprintf(`{ query(source: "%s", input: "select * from table_that_does_not_exist") {
			                                          ... on queryError {
								                        message
								                      }
		                                            }
												  }`, dataSource.Name()),
				code:    200,
				fixture: "query_error.json",
			},
		}

		for _, test := range tests {
			t.Run(test.fixture, func(t *testing.T) {
				req := httptest.NewRequest("POST", "http://example.com/graphql", strings.NewReader(graphqlPayload(t, test.query)))
				w := httptest.NewRecorder()

				api.GraphQLHandler(dataSources)(w, req)

				resp, err := ioutil.ReadAll(w.Body)
				if err != nil {
					t.Fatal(err)
				}
				actual := string(resp)

				if w.Code != test.code {
					t.Fatalf("expected code %d, got: %d. Resp: %s", test.code, w.Code, actual)
				}

				var expected bytes.Buffer
				testutil.ParseFixture(t, &expected, test.fixture, testutil.DBTemplate{Name: dataSource.Name()})
				if *update {
					testutil.WriteFixture(t, test.fixture, actual)
				}

				if !reflect.DeepEqual(strings.TrimSuffix(expected.String(), "\n"), actual) {
					t.Fatalf("Unexpected result, diff: %v", testutil.Diff(expected.String(), actual))
				}
			})
		}
	}
}
