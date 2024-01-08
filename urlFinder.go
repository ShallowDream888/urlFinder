package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

func checkURL(url string, results chan string, sem chan struct{}, bar *progressbar.ProgressBar, timeout time.Duration) {
	defer func() {
		<-sem
	}()

	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: timeout * time.Millisecond,
	}

	resp, err := client.Get(url)
	if err != nil {
		if strings.Contains(err.Error(), url+"/") {
			results <- fmt.Sprintf("%s,301", url)
		}
		results <- fmt.Sprintf("%s,无响应", url)
		bar.Add(1)
		return
	}
	defer resp.Body.Close()

	results <- fmt.Sprintf("%s,%d", url, resp.StatusCode)
	bar.Add(1)
}

func worker(jobs <-chan string, results chan string, sem chan struct{}, bar *progressbar.ProgressBar, timeout time.Duration) {
	for url := range jobs {
		sem <- struct{}{}
		go checkURL(url, results, sem, bar, timeout)
	}
}

func main() {
	maxWorkersPtr := flag.Int("n", 100, "最大工作线程数")
	timeoutPtr := flag.Int("t", 3000, "超时时间（毫秒）")
	urlsFilePtr := flag.String("f", "urls.txt", "包含URL的文件")
	flag.Parse()

	urls := readURLsFromFile(*urlsFilePtr)
	jobs := make(chan string, len(urls))
	results := make(chan string, len(urls))
	var sem = make(chan struct{}, *maxWorkersPtr)

	bar := progressbar.Default(int64(len(urls)))

	for i := 0; i < *maxWorkersPtr; i++ {
		go worker(jobs, results, sem, bar, time.Duration(*timeoutPtr)*time.Millisecond)
	}

	for _, url := range urls {
		jobs <- url
	}
	close(jobs)

	file, err := os.Create("results.csv")
	if err != nil {
		fmt.Println("创建文件时出错:", err)
		return
	}
	defer file.Close()

	for range urls {
		res := <-results
		_, _ = file.WriteString(res + "\n")
	}

	fmt.Println("所有任务已完成！")
}

func readURLsFromFile(filename string) []string {
	var urls []string

	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("打开文件时出错:", err)
		return urls
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		urls = append(urls, strings.TrimSpace(scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("读取文件时出错:", err)
	}

	return urls
}
