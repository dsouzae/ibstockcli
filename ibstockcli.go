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
	"fmt"
	"github.com/fiorix/go-readline"
	"github.com/gofinance/ib"
	"log"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

var shownewline bool = false
var gEnableRTH bool = true
var gEnableGTC bool = true
var gUpdateOverride bool = false
var gCancel bool = true

type ExecutionInfo struct {
	ExecutionData ib.ExecutionData
	Commission    ib.CommissionReport
}

type IBManager struct {
	label       string
	nextOrderid int64
	engine      *ib.Engine
	opts        ib.EngineOptions
	paper       bool
	elog        map[string]*ExecutionInfo
	realtimeMap map[int64]string
}

func NewOrder() (ib.Order, error) {
	order, err := ib.NewOrder()

	if gEnableRTH {
		order.OutsideRTH = true
	}

	order.TIF = "DAY"
	if gEnableGTC {
		order.TIF = "GTC"
	}

	return order, err
}

func NewContract(symbol string) ib.Contract {
	return ib.Contract{
		Symbol:       symbol,
		SecurityType: "STK",
		Exchange:     "SMART",
		Currency:     "USD",
	}
}

func doBuy(mgr *IBManager, symbol string, quantity uint64, market bool, price float64) {
	request := ib.PlaceOrder{
		Contract: NewContract(symbol),
	}

	request.Order, _ = NewOrder()
	request.Order.Action = "BUY"
	request.Order.TotalQty = int64(quantity)

	if market {
		request.Order.OrderType = "MKT"
	} else {
		request.Order.OrderType = "LMT"
		request.Order.LimitPrice = price
	}
	request.SetID(mgr.NextOrderID())

	mgr.engine.Send(&request)
	log.Printf("%s: Sending BUY for %s, quantity %v, - %s - %v", mgr.label, symbol, quantity, request.Order.OrderType, request.Order.LimitPrice)
}

func doSellTrail(mgr *IBManager, symbol string, quantity uint64, trailamount float64) {
	request := ib.PlaceOrder{
		Contract: NewContract(symbol),
	}

	request.Order, _ = NewOrder()
	request.Order.Action = "SELL"
	request.Order.TotalQty = int64(quantity)
	request.Order.OrderType = "TRAIL"
	request.Order.AuxPrice = trailamount
	request.SetID(mgr.NextOrderID())

	mgr.engine.Send(&request)
	log.Printf("%s: Sending SELL for %s, quantity %v, %s - %.2f", mgr.label, symbol, quantity, request.Order.OrderType, request.Order.AuxPrice)
}

func doSellTrailLimit(mgr *IBManager, symbol string, quantity uint64, trailamount float64, stopprice float64, limitoffset float64) {
	request := ib.PlaceOrder{
		Contract: NewContract(symbol),
	}

	request.Order, _ = NewOrder()
	request.Order.Action = "SELL"
	request.Order.TotalQty = int64(quantity)
	request.Order.OrderType = "TRAIL LIMIT"
	request.Order.AuxPrice = trailamount
	request.Order.TrailStopPrice = stopprice
	request.Order.LimitPrice = stopprice - limitoffset
	request.SetID(mgr.NextOrderID())

	mgr.engine.Send(&request)
	log.Printf("%s: Sending SELL for %s, quantity %v, %s - trail:%.2f stop:%.2f", mgr.label, symbol, quantity, request.Order.OrderType, request.Order.AuxPrice, request.Order.TrailStopPrice)
}

func doBracket(mgr *IBManager, symbol string, quantity uint64, buyprice float64, sellprice float64, stopprice float64) {
	var parentid int64

	request := ib.PlaceOrder{
		Contract: NewContract(symbol),
	}

	parentid = mgr.NextOrderID()
	request.SetID(parentid)
	request.Order, _ = NewOrder()
	request.Order.Transmit = false
	request.Order.Action = "BUY"
	request.Order.TotalQty = int64(quantity)
	request.Order.OrderType = "LMT"
	request.Order.LimitPrice = buyprice

	mgr.engine.Send(&request)
	log.Printf("%s: BRK - Sending BUY for %s, quantity %v, - %s - %v", mgr.label, symbol, quantity, request.Order.OrderType, request.Order.LimitPrice)

	request.SetID(mgr.NextOrderID())
	request.Order, _ = NewOrder()
	request.Order.ParentID = parentid
	request.Order.Transmit = false

	request.Order.Action = "SELL"
	request.Order.TotalQty = int64(quantity)
	request.Order.OrderType = "STP"
	request.Order.AuxPrice = stopprice

	mgr.engine.Send(&request)
	log.Printf("%s: BRK - Sending STP for %s, quantity %v, - %s - %v", mgr.label, symbol, quantity, request.Order.OrderType, request.Order.AuxPrice)

	request.SetID(mgr.NextOrderID())
	request.Order, _ = NewOrder()
	request.Order.ParentID = parentid

	request.Order.Action = "SELL"
	request.Order.TotalQty = int64(quantity)
	request.Order.OrderType = "LMT"
	request.Order.LimitPrice = sellprice

	mgr.engine.Send(&request)
	log.Printf("%s: BRK - Sending SELL for %s, quantity %v, - %s - %v", mgr.label, symbol, quantity, request.Order.OrderType, request.Order.LimitPrice)
}

func doBuyTrail(mgr *IBManager, symbol string, quantity uint64, trailamount float64) {
	request := ib.PlaceOrder{
		Contract: NewContract(symbol),
	}

	request.Order, _ = NewOrder()
	request.Order.Action = "BUY"
	request.Order.TotalQty = int64(quantity)
	request.Order.OrderType = "TRAIL"
	request.Order.AuxPrice = trailamount
	request.SetID(mgr.NextOrderID())

	mgr.engine.Send(&request)
	log.Printf("%s: Sending BUY for %s, quantity %v, %s - %.2f", mgr.label, symbol, quantity, request.Order.OrderType, request.Order.AuxPrice)
}

func doBuyTrailLimit(mgr *IBManager, symbol string, quantity uint64, trailamount float64, stopprice float64, limitoffset float64) {
	request := ib.PlaceOrder{
		Contract: NewContract(symbol),
	}

	request.Order, _ = NewOrder()
	request.Order.Action = "BUY"
	request.Order.TotalQty = int64(quantity)
	request.Order.OrderType = "TRAIL LIMIT"
	request.Order.AuxPrice = trailamount
	request.Order.TrailStopPrice = stopprice
	request.Order.LimitPrice = stopprice + limitoffset
	request.SetID(mgr.NextOrderID())

	mgr.engine.Send(&request)
	log.Printf("%s: Sending BUY for %s, quantity %v, %s - trail:%.2f stop:%.2f", mgr.label, symbol, quantity, request.Order.OrderType, request.Order.AuxPrice, request.Order.TrailStopPrice)
}

func doBuyTrailMarketIfTouched(mgr *IBManager, symbol string, quantity uint64, trailamount float64) {
	request := ib.PlaceOrder{
		Contract: NewContract(symbol),
	}

	request.Order, _ = NewOrder()
	request.Order.Action = "BUY"
	request.Order.TotalQty = int64(quantity)
	request.Order.OrderType = "TRAIL MIT"
	request.Order.AuxPrice = trailamount
	request.SetID(mgr.NextOrderID())

	mgr.engine.Send(&request)
	log.Printf("%s: Sending BUY for %s, quantity %v, %s - %.2f", mgr.label, symbol, quantity, request.Order.OrderType, request.Order.AuxPrice)
}

func doSell(mgr *IBManager, symbol string, quantity uint64, market bool, price float64) {
	request := ib.PlaceOrder{
		Contract: NewContract(symbol),
	}

	request.Order, _ = NewOrder()
	request.Order.Action = "SELL"
	request.Order.TotalQty = int64(quantity)

	if market {
		request.Order.OrderType = "MKT"
	} else {
		request.Order.OrderType = "LMT"
		request.Order.LimitPrice = price
	}
	request.SetID(mgr.NextOrderID())

	mgr.engine.Send(&request)
	log.Printf("%s: Sending SELL for %s, quantity %v, - %s - %v", mgr.label, symbol, quantity, request.Order.OrderType, request.Order.LimitPrice)
}

func doStopMarket(mgr *IBManager, symbol string, quantity uint64, stopprice float64) {
	request := ib.PlaceOrder{
		Contract: NewContract(symbol),
	}

	request.Order, _ = NewOrder()
	request.Order.Action = "SELL"
	request.Order.TotalQty = int64(quantity)
	request.Order.OrderType = "STP"
	request.Order.AuxPrice = stopprice

	request.SetID(mgr.NextOrderID())

	mgr.engine.Send(&request)
	log.Printf("%s: Sending STP SELL for %s, quantity %v, - %s - %v", mgr.label, symbol, quantity, request.Order.OrderType, request.Order.AuxPrice)
}

func doRequestRealTimeBars(mgr *IBManager, symbol string) {
	request := ib.RequestRealTimeBars{
		Contract:   NewContract(symbol),
		BarSize:    5,
		WhatToShow: ib.RealTimeTrades,
		UseRTH:     true,
	}

	id := mgr.NextOrderID()
	request.SetID(id)
	mgr.realtimeMap[id] = symbol

	mgr.engine.Send(&request)

	log.Printf("%s: Sending RealTime Bars For %s", mgr.label, symbol)
}

func (m *IBManager) NextOrderID() int64 {
	val := m.nextOrderid

	m.nextOrderid++
	return val
}

func FloatAdjustValue(val float64) float64 {
	if val > 1000000 {
		return 0.0
	}

	return val
}

func engineLoop(ibmanager *IBManager) {
	var engs chan ib.EngineState = make(chan ib.EngineState)
	var rc chan ib.Reply = make(chan ib.Reply)

	// intialize all subscriptions for messages
	ibmanager.engine.SubscribeState(engs)
	ibmanager.engine.SubscribeAll(rc)

	// Get the next order id
	ibmanager.engine.Send(&ib.RequestIDs{})
	//ibmanager.engine.Send(&ib.RequestManagedAccounts{})

	for {
		select {
		case r := <-rc:
			if shownewline {
				//if len(ibmanager.elog) > 0 {
				//fmt.Printf("\n")
				//}
				shownewline = false
			}

			//log.Printf("%s - RECEIVE %v", ibmanager.label, reflect.TypeOf(r))
			switch r.(type) {

			case (*ib.ErrorMessage):
				r := r.(*ib.ErrorMessage)
				log.Printf("%s ID: %v Code:%3d Message:'%v'\n", ibmanager.label, r.ID(), r.Code, r.Message)

			case (*ib.ManagedAccounts):
				r := r.(*ib.ManagedAccounts)
				for _, acct := range r.AccountsList {
					log.Printf("%s: Account %v\n", ibmanager.label, acct)
				}

			case (*ib.Position):
				r := r.(*ib.Position)
				log.Printf("%s: C:%6v P:%10v AvgC:%10.2f\n", ibmanager.label, r.Contract.Symbol, r.Position, r.AverageCost)

			case (*ib.OpenOrder):
				r := r.(*ib.OpenOrder)
				commission := FloatAdjustValue(r.OrderState.Commission)
				maxcommission := FloatAdjustValue(r.OrderState.MaxCommission)
				mincommission := FloatAdjustValue(r.OrderState.MinCommission)
				log.Printf("%s OrderID: %v,%v Status: %-9v Symbol: %-5v Action   : %-4v  Quantity        : %4v %v %v l:%6.2f a:%6.2f c:%4.2f %4.2f/%4.2f\n", ibmanager.label, r.Order.OrderID, r.Order.ParentID, r.OrderState.Status, r.Contract.Symbol, r.Order.Action, r.Order.TotalQty, r.Order.TIF, r.Order.OrderType, r.Order.LimitPrice, r.Order.AuxPrice, commission, mincommission, maxcommission)

			case (*ib.OrderStatus):
				r := r.(*ib.OrderStatus)
				log.Printf("%s OrderID: %v,%v Status: %-9v Filled: %5v Remaining: %5v AverageFillPrice: %6.2f - WH:'%s'\n", ibmanager.label, r.ID(), r.ParentID, r.Status, r.Filled, r.Remaining, r.AverageFillPrice, r.WhyHeld)

			case (*ib.AccountValue):
				r := r.(*ib.AccountValue)
				if r.Currency == "USD" {
					var show bool
					switch r.Key.Key {
					case "AvailableFunds":
						show = true
					case "BuyingPower":
						show = true
					case "TotalCashValue":
						show = true
					case "GrossPositionValue":
						show = true
					case "NetLiquidation":
						show = true
					case "UnrealizedPnL":
						show = true
					case "RealizedPnL":
						show = true
					case "AccruedCash":
						show = true
					default:
						show = false
					}
					if show || gUpdateOverride {
						log.Printf("%s: K:%-26v V:%20v\n", ibmanager.label, r.Key.Key, r.Value)
					}
				}

			case (*ib.PortfolioValue):
				r := r.(*ib.PortfolioValue)
				log.Printf("%s: C:%6v P:%10v AvgC:%10.2f uPNL:%8.2f PNL:%8.2f\n", ibmanager.label, r.Contract.Symbol, r.Position, r.AverageCost, r.UnrealizedPNL, r.RealizedPNL)

			case (*ib.AccountSummary):
				r := r.(*ib.AccountSummary)
				log.Printf("%s: K:%-26v V:%20v\n", ibmanager.label, r.Key.Key, r.Value)

			case (*ib.ExecutionData):
				r := r.(*ib.ExecutionData)
				item, ok := ibmanager.elog[r.Exec.ExecID]
				if !ok {
					item = new(ExecutionInfo)
					ibmanager.elog[r.Exec.ExecID] = item
				}
				item.ExecutionData = *r

			case (*ib.CommissionReport):
				r := r.(*ib.CommissionReport)
				item, ok := ibmanager.elog[r.ExecutionID]
				if !ok {
					item = new(ExecutionInfo)
					ibmanager.elog[r.ExecutionID] = item
				}
				item.Commission = *r

			case (*ib.AccountSummaryEnd):
				r := r.(*ib.AccountSummaryEnd)

				if gCancel {
					req := &ib.CancelAccountSummary{}
					req.SetID(r.ID())
					ibmanager.engine.Send(req)
				}

			case (*ib.ExecutionDataEnd):
				var keys TimeSlice
				for _, k := range ibmanager.elog {
					keys = append(keys, k)
				}

				sort.Sort(keys)

				for _, x := range keys {
					log.Printf("%s: %v %4d %-7s %s %4d %7.2f %4d %7.2f %6.2f %s\n",
						ibmanager.label,
						x.ExecutionData.Exec.Time.Format("15:04:05"),
						x.ExecutionData.Exec.OrderID,
						x.ExecutionData.Contract.Symbol,
						x.ExecutionData.Exec.Side,
						x.ExecutionData.Exec.Shares,
						x.ExecutionData.Exec.Price,
						x.ExecutionData.Exec.CumQty,
						x.ExecutionData.Exec.AveragePrice,
						x.Commission.Commission,
						x.ExecutionData.Exec.Exchange)
				}

			case (*ib.RealtimeBars):
				r := r.(*ib.RealtimeBars)

				symbol, ok := ibmanager.realtimeMap[r.ID()]
				if !ok {
					symbol = ""
				}

				log.Printf("%10s: %v - Open: %10.2f Close: %10.2f Low %10.2f High %10.2f Volume %10.2f Count %10v WAP %10.2f\n",
					symbol,
					time.Unix(r.Time, 0).Format("15:04:05"),
					r.Open,
					r.Close,
					r.Low,
					r.High,
					r.Volume,
					r.Count,
					r.WAP)

			case (*ib.PositionEnd):

			case (*ib.AccountDownloadEnd):
				if gCancel {
					req := &ib.RequestAccountUpdates{}
					req.Subscribe = false
					ibmanager.engine.Send(req)
				}

			case (*ib.OpenOrderEnd):

			case (*ib.ContractDataEnd):

			case (*ib.TickSnapshotEnd):

			case (*ib.AccountUpdateTime):

			case (*ib.NextValidID):
				r := r.(*ib.NextValidID)
				ibmanager.nextOrderid = r.OrderID

			default:
				log.Printf("%s - RECEIVE %v", ibmanager.label, reflect.TypeOf(r))
				log.Printf("%s X %v\n", ibmanager.label, r)
			}
		case newstate := <-engs:
			log.Printf("%s ERROR: %v\n", ibmanager.label, newstate)
			if newstate != ib.EngineExitNormal {
				log.Fatalf("%s ERROR: %v", ibmanager.label, ibmanager.engine.FatalError())
			}
			return
		}
	}
}

type applyManagerFunc func(*IBManager) error

func applyFunc(empty bool, acctselect string, accts []*IBManager, applyFn applyManagerFunc) {
	if !empty && acctselect == "" {
		fmt.Println("Must select an account to buy/sell")
		return
	}
	for _, ac := range accts {
		if acctselect == "" || ac.label == acctselect {
			_ = applyFn(ac)
		}
	}
}

func main() {

	// Output TWS messages to a separate file and use a split screen terminal to show them.
	if false {
		f, err := os.OpenFile("stockcli.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("LOG ERROR: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	log.SetFlags(log.Ltime | log.Lmicroseconds)

	// load configuration from
	config, cerr := LoadConfigFromFile("config.js")
	if cerr != nil {
		log.Fatalf("ERROR loading initial config %v", cerr)
		return
	}

	acct := make([]*IBManager, 0)
	for _, a := range config.Accounts {
		log.Printf("SETUP: %s %v", a.Label, a.Paper)
		acct = append(acct, &IBManager{
			label: a.Label,
			paper: a.Paper,
			opts: ib.EngineOptions{
				Gateway: a.Gateway,
				Client:  a.Client,
			},
			elog: make(map[string]*ExecutionInfo),
		})
	}

	for _, ac := range acct {
		var err error
		ac.engine, err = ib.NewEngine(ac.opts)
		if err != nil {
			log.Fatalf("error creating %s Engine %v ", ac.label, err)
		}
		defer ac.engine.Stop()
		if ac.engine.State() != ib.EngineReady {
			log.Fatalf("%s engine is not ready", ac.label)
		}
		go engineLoop(ac)
	}

	time.Sleep(1 * time.Second)

	acctselect := ""
	prompt := "> "

	// Loop until Readline returns nil (signalling EOF)
	lastresult := ""
L:
	for {
		result := readline.Readline(&prompt)
		if result == nil {
			fmt.Println()
			continue
		}

		// prevent duplicate calls
		if *result == lastresult {
			continue
		}

		lastresult = *result
		strs := strings.Fields(strings.TrimSpace(*result))

		if len(strs) == 0 {
			continue
		}

		if *result != "" {
			readline.AddHistory(*result)
		}

		command := strs[0]

		switch {
		case command == "exit":
			break L // exit loop
		case command == "quit":
			break L // exit loop

		case command == "summary":
			lastresult = ""
			applyFunc(true, acctselect, acct, func(ac *IBManager) error {
				reqAs := &ib.RequestAccountSummary{}
				reqAs.SetID(ac.engine.NextRequestID())
				reqAs.Group = "All"
				reqAs.Tags = "BuyingPower,NetLiquidation,GrossPositionValue,TotalCashValue,SettledCash,InitMarginReq,MaintMarginReq,AvailableFunds,TotalCashValue,UnrealizedPnL"
				ac.engine.Send(reqAs)
				shownewline = true
				return nil
			})

		case command == "open":
			lastresult = ""
			applyFunc(true, acctselect, acct, func(ac *IBManager) error {
				ac.engine.Send(&ib.RequestOpenOrders{})
				shownewline = true
				return nil
			})

		case command == "positions":
			lastresult = ""
			applyFunc(true, acctselect, acct, func(ac *IBManager) error {
				req := &ib.RequestPositions{}
				ac.engine.Send(req)
				shownewline = true
				return nil
			})

		case command == "updates":
			lastresult = ""
			applyFunc(true, acctselect, acct, func(ac *IBManager) error {
				req := &ib.RequestAccountUpdates{}
				req.Subscribe = true
				ac.engine.Send(req)
				shownewline = true
				return nil
			})

		case command == "noupdates":
			lastresult = ""
			applyFunc(true, acctselect, acct, func(ac *IBManager) error {
				req := &ib.RequestAccountUpdates{}
				req.Subscribe = false
				ac.engine.Send(req)
				return nil
			})

		case command == "elog":
			lastresult = ""
			applyFunc(true, acctselect, acct, func(ac *IBManager) error {
				ac.elog = make(map[string]*ExecutionInfo)
				ereq := ib.RequestExecutions{}
				ereq.SetID(ac.engine.NextRequestID())
				ac.engine.Send(&ereq)
				shownewline = true
				return nil
			})

		case command == "select":
			lastresult = ""
			if len(strs) != 2 {
				fmt.Printf("select <label|all>\n")
				continue
			}
			if strs[1] == "all" {
				acctselect = ""
				prompt = "> "
			} else {
				for _, ac := range acct {
					if ac.label == strs[1] {
						acctselect = ac.label
						prompt = acctselect + " > "
						break
					}
				}
			}

		case command == "sell-t":
			if len(strs) != 4 {
				fmt.Printf("sell-t <symbol> <quantity> <trailamount>\n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)
			trailamount, _ := strconv.ParseFloat(strs[3], 64)

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doSellTrail(ac, strs[1], quantity, trailamount)
				shownewline = true
				return nil
			})

		case command == "sell-tl":
			if len(strs) != 6 {
				fmt.Printf("sell-tl <symbol> <quantity> <stopprice> <trailamount> <limitoffset>\n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)
			stopprice, _ := strconv.ParseFloat(strs[3], 64)
			trailamount, _ := strconv.ParseFloat(strs[4], 64)
			limitoffset, _ := strconv.ParseFloat(strs[5], 64)

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doSellTrailLimit(ac, strs[1], quantity, trailamount, stopprice, limitoffset)
				shownewline = true
				return nil
			})

		case command == "sell-l":
			if len(strs) != 4 {
				fmt.Printf("sell-l <symbol> <quantity> <limitprice>\n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)
			limitprice, _ := strconv.ParseFloat(strs[3], 64)

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doSell(ac, strs[1], quantity, false, limitprice)
				shownewline = true
				return nil
			})

		case command == "sell-m":
			if len(strs) != 3 {
				fmt.Printf("sell-m <symbol> <quantity>\n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doSell(ac, strs[1], quantity, true, 0)
				shownewline = true
				return nil
			})

		case command == "buy-t":
			if len(strs) != 4 {
				fmt.Printf("buy-t <symbol> <quantity> <trailamount>\n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)
			trailamount, _ := strconv.ParseFloat(strs[3], 64)

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doBuyTrail(ac, strs[1], quantity, trailamount)
				shownewline = true
				return nil
			})

		case command == "buy-if":
			if len(strs) != 4 {
				fmt.Printf("buy-if <symbol> <quantity> <trailamount>\n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)
			trailamount, _ := strconv.ParseFloat(strs[3], 64)

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doBuyTrailMarketIfTouched(ac, strs[1], quantity, trailamount)
				shownewline = true
				return nil
			})

		case command == "buy-tl":
			if len(strs) != 6 {
				fmt.Printf("buy-tl <symbol> <quantity> <stopprice> <trailamount> <limitoffset>\n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)
			stopprice, _ := strconv.ParseFloat(strs[3], 64)
			trailamount, _ := strconv.ParseFloat(strs[4], 64)
			limitoffset, _ := strconv.ParseFloat(strs[5], 64)

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doBuyTrailLimit(ac, strs[1], quantity, trailamount, stopprice, limitoffset)
				shownewline = true
				return nil
			})

		case command == "buy-l":
			if len(strs) != 4 {
				fmt.Printf("buy-l <symbol> <quantity> <limitprice>\n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)
			limitprice, _ := strconv.ParseFloat(strs[3], 64)

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doBuy(ac, strs[1], quantity, false, limitprice)
				shownewline = true
				return nil
			})

		case command == "buy-m":
			if len(strs) != 3 {
				fmt.Printf("buy-m <symbol> <quantity>\n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doBuy(ac, strs[1], quantity, true, 0)
				shownewline = true
				return nil
			})

		case command == "bracket":
			if len(strs) != 6 {
				fmt.Printf("bracket <symbol> <quantity> <buyprice> <sellprice> <stopprice>\n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)
			buyprice, _ := strconv.ParseFloat(strs[3], 64)
			sellprice, _ := strconv.ParseFloat(strs[4], 64)
			stopprice, _ := strconv.ParseFloat(strs[5], 64)

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doBracket(ac, strs[1], quantity, buyprice, sellprice, stopprice)
				shownewline = true
				return nil
			})

		case command == "brka":
			if len(strs) != 6 {
				fmt.Printf("brka <symbol> <quantity> <buyprice> <selloff> <stopoff>\n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)
			buyprice, _ := strconv.ParseFloat(strs[3], 64)
			sellprice, _ := strconv.ParseFloat(strs[4], 64)
			stopprice, _ := strconv.ParseFloat(strs[5], 64)

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doBracket(ac, strs[1], quantity, buyprice, buyprice+sellprice, buyprice-stopprice)
				shownewline = true
				return nil
			})

		case command == "brkp1":
			if len(strs) != 4 {
				fmt.Printf("brkp1 <symbol> <quantity> <buyprice> {sell = buy + 0.20, stp = buy - 0.05} \n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)
			buyprice, _ := strconv.ParseFloat(strs[3], 64)
			sellprice := buyprice + 0.20
			stopprice := buyprice - 0.05

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doBracket(ac, strs[1], quantity, buyprice, sellprice, stopprice)
				shownewline = true
				return nil
			})

		case command == "brkp2":
			if len(strs) != 4 {
				fmt.Printf("brkp2 <symbol> <quantity> <buyprice> {sell = buy + 0.11, stp = buy - 0.05} \n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)
			buyprice, _ := strconv.ParseFloat(strs[3], 64)
			sellprice := buyprice + 0.11
			stopprice := buyprice - 0.05

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doBracket(ac, strs[1], quantity, buyprice, sellprice, stopprice)
				shownewline = true
				return nil
			})

		case command == "stop-m":
			if len(strs) != 4 {
				fmt.Printf("stop-m <symbol> <quantity> <stopprice>\n")
				continue
			}

			quantity, _ := strconv.ParseUint(strs[2], 10, 64)
			stopprice, _ := strconv.ParseFloat(strs[3], 64)

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doStopMarket(ac, strs[1], quantity, stopprice)
				shownewline = true
				return nil
			})

		case command == "override":
			if len(strs) != 2 {
				fmt.Printf("override status %v\n", gUpdateOverride)
				continue
			}

			if strs[1] == "on" {
				gUpdateOverride = true
			} else {
				gUpdateOverride = false
			}
			fmt.Printf("override %v\n", gUpdateOverride)
		case command == "rth":
			if len(strs) != 2 {
				fmt.Printf("rth status %v\n", gEnableRTH)
				continue
			}

			if strs[1] == "on" {
				gEnableRTH = true
			} else {
				gEnableRTH = false
			}
			fmt.Printf("rth status %v\n", gEnableRTH)

		case command == "gtc":
			if len(strs) != 2 {
				fmt.Printf("gtc status %v\n", gEnableGTC)
				continue
			}

			if strs[1] == "on" {
				gEnableGTC = true
			} else {
				gEnableGTC = false
			}
			fmt.Printf("gtc status %v\n", gEnableGTC)

		case command == "acct-cancel":
			if len(strs) != 2 {
				fmt.Printf("acct-cancel status %v\n", gCancel)
				continue
			}

			if strs[1] == "on" {
				gCancel = true
			} else {
				gCancel = false
			}
			fmt.Printf("acct-cancel status %v\n", gCancel)

		case command == "realtimebar":
			if len(strs) != 2 {
				fmt.Printf("realtimebar <symbol>\n")
				continue
			}

			applyFunc(false, acctselect, acct, func(ac *IBManager) error {
				doRequestRealTimeBars(ac, strs[1])
				return nil
			})

		case command == "cancel":
			lastresult = ""
			if len(strs) != 2 {
				fmt.Printf("cancel <orderid>\n")
				continue
			}
			orderid := int64(0)
			if strs[1] != "all" {
				orderid, _ = strconv.ParseInt(strs[1], 10, 64)
			}

			applyFunc(true, acctselect, acct, func(ac *IBManager) error {
				if strs[1] == "all" {
					ac.engine.Send(&ib.RequestGlobalCancel{})
				} else {
					request := ib.CancelOrder{}
					request.SetID(orderid)
					ac.engine.Send(&request)
					shownewline = true
				}
				return nil
			})

		case command == "cancelall":
			lastresult = ""

			applyFunc(true, acctselect, acct, func(ac *IBManager) error {
				ac.engine.Send(&ib.RequestGlobalCancel{})
				shownewline = true
				return nil
			})

		case command != "": // Ignore blank lines
			fmt.Println(*result)
		}
	}
}
