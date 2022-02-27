package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

type User struct {
	Name     string
	Email    string
	Browsers []string
}

// вам надо написать более быструю оптимальную этой функции
func FastSearch(out io.Writer) {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}

	fileScanner := bufio.NewScanner(file)
	fileScanner.Split(bufio.ScanLines)

	r := regexp.MustCompile("@")
	seenBrowsers := make(map[string]struct{}, 256)
	uniqueBrowsers := 0
	foundUsers := ""

	for i := 0; fileScanner.Scan(); i++ {
		line := fileScanner.Text()
		// fmt.Printf("%v %v\n", err, line)
		user := User{}
		err := json.Unmarshal([]byte(line), &user)
		if err != nil {
			panic(err)
		}

		isAndroid := false
		isMSIE := false

		for _, browser := range user.Browsers {
			if strings.Contains(browser, "Android") {
				isAndroid = true
				_, seenBefore := seenBrowsers[browser]

				if !seenBefore {
					// log.Printf("SLOW New browser: %s, first seen: %s", browser, user["name"])
					seenBrowsers[browser] = struct{}{}
					uniqueBrowsers++
				}
			}
		}

		for _, browser := range user.Browsers {
			if strings.Contains(browser, "MSIE") {
				isMSIE = true
				_, seenBefore := seenBrowsers[browser]

				if !seenBefore {
					// log.Printf("SLOW New browser: %s, first seen: %s", browser, user["name"])
					seenBrowsers[browser] = struct{}{}
					uniqueBrowsers++
				}
			}
		}

		if !(isAndroid && isMSIE) {
			continue
		}

		// log.Println("Android and MSIE user:", user["name"], user["email"])
		email := r.ReplaceAllString(user.Email, " [at] ")
		foundUsers += fmt.Sprintf("[%d] %s <%s>\n", i, user.Name, email)
	}

	fmt.Fprintln(out, "found users:\n"+foundUsers)
	fmt.Fprintln(out, "Total unique browsers", len(seenBrowsers))
}
