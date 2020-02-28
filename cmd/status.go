/*Package cmd contains all commands.
Copyright © 2019 Mikael Fridh <frimik@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/frimik/auroractl/pkg/format"
	"github.com/frimik/auroractl/pkg/util"

	"github.com/gookit/color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:     "status [/path/to/file.aurora]",
	Short:   "Summarize status for all jobs in aurora file.",
	Example: " status /jobs/app.aurora",
	Args:    cobra.MinimumNArgs(1),
	RunE:    statusCmdF,
}

func init() {
	rootCmd.AddCommand(statusCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// statusCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// statusCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}

// JobUpdate contains info on a potential job update
type JobUpdate struct {
	JobIndex    int
	Job         Job
	Dirty       bool
	Diff        []string
	FoundHeader bool
	Update      string
	Add         string
	Remove      string
}

// Job contains a job
type Job struct {
	Cluster string
	Role    string
	Env     string
	Job     string
	JobPath string
}

// NewJobFromString returns a Job struct based on "cluster/role/env/job" string
func NewJobFromString(job string) Job {
	jobParts := strings.Split(job, "/")

	return Job{
		Cluster: jobParts[0],
		Role:    jobParts[1],
		Env:     jobParts[2],
		Job:     jobParts[3],
		JobPath: job,
	}
}

// NewJobUpdate initializes a new JobUpdate with a default Dirty value set to false
func NewJobUpdate(job string, jobindex int) JobUpdate {
	jobupdate := JobUpdate{}
	jobupdate.Job = NewJobFromString(job)
	jobupdate.JobIndex = jobindex
	// set defaults
	jobupdate.Dirty = false
	jobupdate.FoundHeader = false

	return jobupdate
}

func verboseFlag() string {
	return "--verbose"
}

func statusCmdF(command *cobra.Command, args []string) error {

	// unified is a nicer diff view
	// ignore `owner` differences
	os.Setenv("DIFF_VIEWER", "diff -u -I \"'owner': Identity\"")

	auroraExe := "aurora"

	auroraExePath, err := exec.LookPath(auroraExe)
	if err != nil {
		log.Fatalf("%s not found", auroraExe)
	}
	log.Infof("%s is available at %s\n", auroraExe, auroraExePath)

	auroraFiles := args
	//auroraFile := args[0]

	for i := range auroraFiles {
		auroraFile := auroraFiles[i]

		argParts := []string{"config", "list"}
		if debug {
			argParts = append(argParts, verboseFlag())
		}
		argParts = append(argParts, auroraFile)
		listCmd := exec.Command(auroraExePath, argParts...)

		var out bytes.Buffer
		listCmd.Stdout = &out
		listCmd.Stderr = os.Stderr
		err = listCmd.Run()

		if err != nil {
			log.Errorf("%v: %v", auroraFile, err)
		}

		str1 := out.String()
		re := regexp.MustCompile(`\[([^\[\]]*)\]`)
		submatch := re.FindString(str1)

		jobseparators := "[], "
		f := func(r rune) bool {
			return strings.ContainsRune(jobseparators, r)
		}

		jobs := strings.FieldsFunc(submatch, f)
		//log.Infof("Jobs: %q\n", jobs)

		log.Infof("Aurora file: %s contains %d jobs.", auroraFile, len(jobs))

		var filteredJobs []JobUpdate

		for i, job := range jobs {
			jobSelected := true

			j := NewJobUpdate(job, i)

			if len(auroraRoles) > 0 || len(auroraEnvs) > 0 || len(auroraJobs) > 0 {
				// do filtering
				jobSelected = false
				if len(auroraRoles) > 0 && util.StringInSlice(j.Job.Role, auroraRoles) {
					jobSelected = true
				}
				if len(auroraEnvs) > 0 && util.StringInSlice(j.Job.Env, auroraEnvs) {
					jobSelected = true
				}
				if len(auroraJobs) > 0 && util.StringInSlice(j.Job.Job, auroraJobs) {
					jobSelected = true
				}
			}

			if jobSelected {
				filteredJobs = append(filteredJobs, j)
			}
		}

		log.Infof("Aurora file: %s contains %d jobs after filtering.", auroraFile, len(filteredJobs))

		for _, j := range filteredJobs {
			argParts := []string{"job", "diff"}
			if debug {
				argParts = append(argParts, verboseFlag())
			}
			argParts = append(argParts, j.Job.JobPath, auroraFile)
			diffCmd := exec.Command(auroraExePath, argParts...)
			diffCmd.Env = append(os.Environ(), "AURORA_UNATTENDED=1")

			updateCmdString := fmt.Sprintf("%s update start %s %s", auroraExe, j.Job.JobPath, auroraFile)

			var out bytes.Buffer
			diffCmd.Stdout = &out
			diffCmd.Stderr = os.Stderr
			err = diffCmd.Run()
			if err != nil {
				log.Errorf("%v: Error running diff Command, possibly it expects input on stdin: %v", j.Job.JobPath, err)
				continue
			}

			//fmt.Printf("%q\n", out.String())

			scanner := bufio.NewScanner(strings.NewReader(out.String()))

			for l := 0; scanner.Scan(); l++ {
				//fmt.Println(l, scanner.Text())
				if strings.HasPrefix(scanner.Text(), "This job update will:") {
					j.FoundHeader = true
				}

				if strings.HasPrefix(scanner.Text(), "remove instances:") {
					j.Dirty = true
					instances := re.FindString(scanner.Text())
					j.Remove = instances
				} else if strings.HasPrefix(scanner.Text(), "add instances:") {
					j.Dirty = true
					instances := re.FindString(scanner.Text())
					j.Add = instances
				} else if strings.HasPrefix(scanner.Text(), "update instances:") {
					instances := re.FindString(scanner.Text())
					j.Update = instances
				} else if !j.FoundHeader {
					j.Diff = append(j.Diff, scanner.Text())
					j.Dirty = true
				}

			}

			// format if dirty
			statusFormat := format.Update("# Job [%d: %s]: ") + "Dirty: %t. Remove: " + format.Remove("%s") + ", Add: " + format.Add("%s") + ", Update: " + format.Update("%s") + ". (Diff: %d lines)\n"

			if j.Dirty {
				color.Printf(statusFormat, j.JobIndex, j.Job.JobPath, j.Dirty, j.Remove, j.Add, j.Update, len(j.Diff))
				color.Warn.Println(updateCmdString)

			} else {
				statusFormat := format.LightGreen("# Job [%d: %s]: ") + "Dirty: %t. Remove: " + format.Remove("%s") + ", Add: " + format.Add("%s") + ", Update: " + format.Update("%s") + ". (Diff: %d lines)\n"
				color.Printf(statusFormat, j.JobIndex, j.Job.JobPath, j.Dirty, j.Remove, j.Add, j.Update, len(j.Diff))
			}

			if verbose {
				for _, l := range j.Diff {
					fmt.Println(l)
				}
			}
		}

	}

	/* `aurora job diff` output sample:
		This job update will:
	update instances: [0-2]
	with diff:

	59c59,60
	<   -com.twitter.finagle.netty3.numWorkers=3 \\\\\
	---
	>   -com.twitter.finagle.netty3.numWorkers=6 \\\\\
	*/

	return nil
}
