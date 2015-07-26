/* ibstockcli - A command line program to interact with the IB TWS API using the gofinance/ib library
 *
 * Copyright (C) 2015 Ellery D'Souza <edsouza99@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"encoding/json"
	"os"
)

type Account struct {
	Label   string
	Gateway string
	Client  int64
	Paper   bool
}

type Config struct {
	Accounts []Account
}

func LoadConfigFromFile(filename string) (*Config, error) {
	// Open the file.
	file, err := os.Open(filename)

	if err != nil {
		return nil, err
	}

	defer file.Close()

	config := Config{}
	err = json.NewDecoder(file).Decode(&config)

	return &config, err
}
