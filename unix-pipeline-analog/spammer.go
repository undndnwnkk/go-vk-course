package main

import (
	"cmp"
	"slices"
	"strconv"
	"sync"
)

func RunPipeline(cmds ...cmd) {
	n := len(cmds) + 1
	channels := make([]chan interface{}, n)

	for i := range n {
		newChan := make(chan interface{})
		channels[i] = newChan
	}

	var wg sync.WaitGroup

	for i := range cmds {
		wg.Add(1)
		curCmd := cmds[i]
		in := channels[i]
		out := channels[i+1]
		go func(curCmd cmd, in, out chan interface{}) {
			curCmd(in, out)
			close(out)
			wg.Done()
		}(curCmd, in, out)
	}

	wg.Wait()
}

func SelectUsers(in, out chan interface{}) {
	// 	in - string
	// 	out - User

	var idSet = make(map[uint64]struct{}, 0)
	var wg sync.WaitGroup
	var mux sync.Mutex
	for val := range in {
		email := val.(string)
		var add bool

		wg.Add(1)
		go func() {

			defer wg.Done()
			user := GetUser(email)
			mux.Lock()
			if _, ok := idSet[user.ID]; !ok {
				idSet[user.ID] = struct{}{}
				add = true
			}
			mux.Unlock()

			if add {
				out <- user
			}
		}()
	}

	wg.Wait()
}

func SelectMessages(in, out chan interface{}) {
	// 	in - User
	// 	out - MsgID

	commonBatch := make([]User, 0, 2)
	var wg sync.WaitGroup

	for val := range in {
		usr := val.(User)
		commonBatch = append(commonBatch, usr)

		if len(commonBatch) != 2 {
			continue
		}

		workerBatch := slices.Clone(commonBatch)
		commonBatch = commonBatch[:0]

		wg.Add(1)
		go func() {
			defer wg.Done()
			msgIDs, _ := GetMessages(workerBatch[0], workerBatch[1])

			for _, i := range msgIDs {
				out <- i
			}
		}()
	}

	if len(commonBatch) == 1 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msgIDs, _ := GetMessages(commonBatch[0])

			for _, i := range msgIDs {
				out <- i
			}
		}()
	}

	wg.Wait()
}

func CheckSpam(in, out chan interface{}) {
	// in - MsgID
	// out - MsgData

	var wg sync.WaitGroup
	spamChan := make(chan struct{}, HasSpamMaxAsyncRequests)

	for v := range in {
		id := v.(MsgID)
		spamChan <- struct{}{}

		wg.Add(1)
		go func() {
			defer wg.Done()
			spam, _ := HasSpam(id)
			<-spamChan

			data := MsgData{ID: id, HasSpam: spam}
			out <- data
		}()
	}

	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	// in - MsgData
	// out - string

	data := make([]MsgData, 0)
	for v := range in {
		cur := v.(MsgData)
		data = append(data, cur)
	}

	slices.SortFunc(data, func(a, b MsgData) int {
		if a.HasSpam && !b.HasSpam {
			return -1
		}

		if !a.HasSpam && b.HasSpam {
			return 1
		}

		return cmp.Compare(a.ID, b.ID)
	})

	for _, v := range data {
		res := strconv.FormatBool(v.HasSpam) + " " + strconv.FormatUint(uint64(v.ID), 10)
		out <- res
	}
}
