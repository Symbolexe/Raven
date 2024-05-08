package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

var (
	internalURLs = make(map[string]bool)
	externalURLs = make(map[string]bool)
	maxURLs      int
	maxDepth     int
	wg           sync.WaitGroup
	mutex        sync.Mutex
	concurrency  int
	client       = &http.Client{
		Timeout: 10 * time.Second,
	}
)

func isValidURL(u string) bool {
	parsed, err := url.Parse(u)
	if err != nil {
		return false
	}
	return parsed.Scheme != "" && parsed.Host != ""
}

func getAllWebsiteLinks(u string) []string {
	var urls []string
	resp, err := client.Get(u)
	if err != nil {
		log.Printf("[raven] Error fetching URL %s: %v\n", u, err)
		return urls
	}
	defer resp.Body.Close()

	tokenizer := html.NewTokenizer(resp.Body)
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			break
		}
		token := tokenizer.Token()
		if tokenType == html.StartTagToken && token.Data == "a" {
			for _, attr := range token.Attr {
				if attr.Key == "href" {
					href := attr.Val
					if href == "" {
						continue
					}
					if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "tel:") {
						continue
					}
					if !isValidURL(href) {
						continue
					}
					urls = append(urls, href)
				}
			}
		}
	}
	return urls
}

func crawl(u string, depth int) {
	defer wg.Done()
	if depth > maxDepth {
		return
	}
	log.Printf("[raven] [*] Crawling: %s (Depth: %d)\n", u, depth)
	links := getAllWebsiteLinks(u)
	mutex.Lock()
	for _, link := range links {
		if len(internalURLs)+len(externalURLs) > maxURLs {
			break
		}
		_, isInternal := internalURLs[link]
		_, isExternal := externalURLs[link]
		if isInternal || isExternal {
			continue
		}
		if strings.Contains(link, u) {
			log.Printf("[raven] [*] Internal link: %s\n", link)
			internalURLs[link] = true
		} else {
			log.Printf("[raven] [!] External link: %s\n", link)
			externalURLs[link] = true
		}
		wg.Add(1)
		go crawl(link, depth+1)
	}
	mutex.Unlock()
}

func main() {
	flag.IntVar(&maxURLs, "maxURLs", 100, "Maximum number of URLs to crawl")
	flag.IntVar(&maxDepth, "maxDepth", 3, "Maximum depth of crawling")
	flag.IntVar(&concurrency, "concurrency", 10, "Number of concurrent requests")
	flag.Parse()

	startURL := flag.Arg(0)
	if startURL == "" {
		fmt.Println("Usage: raven [options] [URL]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	wg.Add(1)
	go crawl(startURL, 0)
	wg.Wait()

	fmt.Println("[+] Total Internal links:", len(internalURLs))
	fmt.Println("[+] Total External links:", len(externalURLs))
	fmt.Println("[+] Total URLs:", len(internalURLs)+len(externalURLs))
}
