package junit

import (
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/report"
)

func TestReporter_Render(t *testing.T) {
	type fields struct {
		TestResults []report.TestResult
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "all pass",
			fields: fields{
				TestResults: []report.TestResult{
					{
						Name:     "Firefox",
						Duration: 34479 * time.Millisecond,
						Status:   job.StatePassed,
						Browser:  "Firefox",
						Platform: "Windows 10",
						URL:      "https://app.saucelabs.com/tests/1234",
					},
					{
						Name:     "Chrome",
						Duration: 5123 * time.Millisecond,
						Status:   job.StatePassed,
						Browser:  "Chrome",
						Platform: "Windows 10",
					},
				},
			},
			want: `<testsuites>
  <testsuite name="Firefox" tests="0" time="34">
    <properties>
      <property name="url" value="https://app.saucelabs.com/tests/1234"></property>
      <property name="browser" value="Firefox"></property>
      <property name="platform" value="Windows 10"></property>
    </properties>
  </testsuite>
  <testsuite name="Chrome" tests="0" time="5">
    <properties>
      <property name="browser" value="Chrome"></property>
      <property name="platform" value="Windows 10"></property>
    </properties>
  </testsuite>
</testsuites>
`,
		},
		{
			name: "with failure",
			fields: fields{
				TestResults: []report.TestResult{
					{
						Name:     "Firefox",
						Duration: 34479 * time.Millisecond,
						Status:   job.StatePassed,
						Browser:  "Firefox",
						Platform: "Windows 10",
					},
					{
						Name:     "Chrome",
						Duration: 171452 * time.Millisecond,
						Status:   job.StateFailed,
						Browser:  "Chrome",
						Platform: "Windows 10",
					},
				},
			},
			want: `<testsuites>
  <testsuite name="Firefox" tests="0" time="34">
    <properties>
      <property name="browser" value="Firefox"></property>
      <property name="platform" value="Windows 10"></property>
    </properties>
  </testsuite>
  <testsuite name="Chrome" tests="0" time="171">
    <properties>
      <property name="browser" value="Chrome"></property>
      <property name="platform" value="Windows 10"></property>
    </properties>
  </testsuite>
</testsuites>
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.CreateTemp("", "saucectl-report.xml")
			if err != nil {
				t.Fatalf("Failed to create temp file %s", err)
			}
			//f.Close()
			defer os.Remove(f.Name())

			r := &Reporter{
				TestResults: tt.fields.TestResults,
				Filename:    f.Name(),
			}
			r.Render()

			b, err := io.ReadAll(f)
			if err != nil {
				t.Fatalf("Failed to read and verify output file %s due to %s", f.Name(), err)
			}

			bstr := string(b)
			if !reflect.DeepEqual(bstr, tt.want) {
				t.Errorf("Render() got = \n%s, want = \n%s", bstr, tt.want)
			}
		})
	}
}

func TestReduceJunitFiles(t *testing.T) {
	input := []junit.TestSuites{
		{
			TestSuites: []junit.TestSuite{
				{
					Tests:   24,
					Errors:  2,
					Time:    "47.917",
					Package: "com.example.android.testing.espresso.BasicSample",
					TestCases: []junit.TestCase{
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test10Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test10Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test11Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test11Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
							Status:    "error",
							Error:     "androidx.test.espresso.base.AssertionErrorHandler$AssertionFailedWithCauseError: 'an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"' doesn't match the selected view.\nExpected: an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"\n     Got: view.getText() was \"INVALID TYPING\"\nView Details: TextView{id=2130903044, res-name=textToBeChanged, visibility=VISIBLE, width=328, height=59, has-focus=false, has-focusable=false, has-window-focus=true, is-clickable=false, is-enabled=true, is-focused=false, is-focusable=false, is-layout-requested=false, is-selected=false, layout-params=android.widget.LinearLayout$LayoutParams@YYYYYY, tag=null, root-is-layout-requested=false, has-input-connection=false, x=220.0, y=96.0, text=INVALID TYPING, input-type=0, ime-target=false, has-links=false}\n\n\tat dalvik.system.VMStack.getThreadStackTrace(Native Method)\n\tat java.lang.Thread.getStackTrace(Thread.java:1841)\n\tat androidx.test.espresso.base.AssertionErrorHandler.handleSafely(AssertionErrorHandler.java:35)\n\tat androidx.test.espresso.base.AssertionErrorHandler.handleSafely(AssertionErrorHandler.java:26)\n\tat androidx.test.espresso.base.DefaultFailureHandler$TypedFailureHandler.handle(DefaultFailureHandler.java:167)\n\tat androidx.test.espresso.base.DefaultFailureHandler.handle(DefaultFailureHandler.java:128)\n\tat androidx.test.espresso.ViewInteraction.waitForAndHandleInteractionResults(ViewInteraction.java:388)\n\tat androidx.test.espresso.ViewInteraction.check(ViewInteraction.java:367)\n\tat com.example.android.testing.espresso.BasicSample.Test12Test.changeText_sameActivity(Test12Test.java:74)\n\t... 33 trimmed\nCaused by: junit.framework.AssertionFailedError: 'an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"' doesn't match the selected view.\nExpected: an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"\n     Got: view.getText() was \"INVALID TYPING\"\nView Details: TextView{id=2130903044, res-name=textToBeChanged, visibility=VISIBLE, width=328, height=59, has-focus=false, has-focusable=false, has-window-focus=true, is-clickable=false, is-enabled=true, is-focused=false, is-focusable=false, is-layout-requested=false, is-selected=false, layout-params=android.widget.LinearLayout$LayoutParams@YYYYYY, tag=null, root-is-layout-requested=false, has-input-connection=false, x=220.0, y=96.0, text=INVALID TYPING, input-type=0, ime-target=false, has-links=false}\n\n\tat androidx.test.espresso.matcher.ViewMatchers.assertThat(ViewMatchers.java:611)\n\tat androidx.test.espresso.assertion.ViewAssertions$MatchesViewAssertion.check(ViewAssertions.java:97)\n\tat androidx.test.espresso.ViewInteraction$SingleExecutionViewAssertion.check(ViewInteraction.java:489)\n\tat androidx.test.espresso.ViewInteraction$2.call(ViewInteraction.java:347)\n\tat androidx.test.espresso.ViewInteraction$2.call(ViewInteraction.java:320)\n\tat java.util.concurrent.FutureTask.run(FutureTask.java:264)\n\tat android.os.Handler.handleCallback(Handler.java:942)\n\tat android.os.Handler.dispatchMessage(Handler.java:99)\n\tat android.os.Looper.loopOnce(Looper.java:201)\n\tat android.os.Looper.loop(Looper.java:288)\n\tat android.app.ActivityThread.main(ActivityThread.java:7872)\n\tat java.lang.reflect.Method.invoke(Native Method)\n\tat com.android.internal.os.RuntimeInit$MethodAndArgsCaller.run(RuntimeInit.java:548)\n\tat com.android.internal.os.ZygoteInit.main(ZygoteInit.java:936)",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test1Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test1Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test2Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test2Test",
							Status:    "error",
							Error:     "androidx.test.espresso.base.AssertionErrorHandler$AssertionFailedWithCauseError: 'an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"' doesn't match the selected view.\nExpected: an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"\n     Got: view.getText() was \"INVALID TYPING\"\nView Details: TextView{id=2130903044, res-name=textToBeChanged, visibility=VISIBLE, width=328, height=59, has-focus=false, has-focusable=false, has-window-focus=true, is-clickable=false, is-enabled=true, is-focused=false, is-focusable=false, is-layout-requested=false, is-selected=false, layout-params=android.widget.LinearLayout$LayoutParams@YYYYYY, tag=null, root-is-layout-requested=false, has-input-connection=false, x=220.0, y=96.0, text=INVALID TYPING, input-type=0, ime-target=false, has-links=false}\n\n\tat dalvik.system.VMStack.getThreadStackTrace(Native Method)\n\tat java.lang.Thread.getStackTrace(Thread.java:1841)\n\tat androidx.test.espresso.base.AssertionErrorHandler.handleSafely(AssertionErrorHandler.java:35)\n\tat androidx.test.espresso.base.AssertionErrorHandler.handleSafely(AssertionErrorHandler.java:26)\n\tat androidx.test.espresso.base.DefaultFailureHandler$TypedFailureHandler.handle(DefaultFailureHandler.java:167)\n\tat androidx.test.espresso.base.DefaultFailureHandler.handle(DefaultFailureHandler.java:128)\n\tat androidx.test.espresso.ViewInteraction.waitForAndHandleInteractionResults(ViewInteraction.java:388)\n\tat androidx.test.espresso.ViewInteraction.check(ViewInteraction.java:367)\n\tat com.example.android.testing.espresso.BasicSample.Test12Test.changeText_sameActivity(Test12Test.java:74)\n\t... 33 trimmed\nCaused by: junit.framework.AssertionFailedError: 'an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"' doesn't match the selected view.\nExpected: an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"\n     Got: view.getText() was \"INVALID TYPING\"\nView Details: TextView{id=2130903044, res-name=textToBeChanged, visibility=VISIBLE, width=328, height=59, has-focus=false, has-focusable=false, has-window-focus=true, is-clickable=false, is-enabled=true, is-focused=false, is-focusable=false, is-layout-requested=false, is-selected=false, layout-params=android.widget.LinearLayout$LayoutParams@YYYYYY, tag=null, root-is-layout-requested=false, has-input-connection=false, x=220.0, y=96.0, text=INVALID TYPING, input-type=0, ime-target=false, has-links=false}\n\n\tat androidx.test.espresso.matcher.ViewMatchers.assertThat(ViewMatchers.java:611)\n\tat androidx.test.espresso.assertion.ViewAssertions$MatchesViewAssertion.check(ViewAssertions.java:97)\n\tat androidx.test.espresso.ViewInteraction$SingleExecutionViewAssertion.check(ViewInteraction.java:489)\n\tat androidx.test.espresso.ViewInteraction$2.call(ViewInteraction.java:347)\n\tat androidx.test.espresso.ViewInteraction$2.call(ViewInteraction.java:320)\n\tat java.util.concurrent.FutureTask.run(FutureTask.java:264)\n\tat android.os.Handler.handleCallback(Handler.java:942)\n\tat android.os.Handler.dispatchMessage(Handler.java:99)\n\tat android.os.Looper.loopOnce(Looper.java:201)\n\tat android.os.Looper.loop(Looper.java:288)\n\tat android.app.ActivityThread.main(ActivityThread.java:7872)\n\tat java.lang.reflect.Method.invoke(Native Method)\n\tat com.android.internal.os.RuntimeInit$MethodAndArgsCaller.run(RuntimeInit.java:548)\n\tat com.android.internal.os.ZygoteInit.main(ZygoteInit.java:936)",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test3Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test3Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test4Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test4Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test5Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test5Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test6Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test6Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test7Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test7Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test8Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test8Test",
							Status:    "success",
						},
						{
							Name:      "changeText_newActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test9Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test9Test",
							Status:    "success",
						},
					},
					SystemOut: "<Base System Output>",
				},
			},
		},
		{
			TestSuites: []junit.TestSuite{
				{
					Tests:   2,
					Errors:  1,
					Time:    "11.007",
					Package: "com.example.android.testing.espresso.BasicSample",
					TestCases: []junit.TestCase{
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test2Test",
							Status:    "success",
						},
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
							Status:    "error",
							Error:     "androidx.test.espresso.base.AssertionErrorHandler$AssertionFailedWithCauseError: 'an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"' doesn't match the selected view.\nExpected: an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"\n     Got: view.getText() was \"INVALID TYPING\"\nView Details: TextView{id=2130903044, res-name=textToBeChanged, visibility=VISIBLE, width=328, height=59, has-focus=false, has-focusable=false, has-window-focus=true, is-clickable=false, is-enabled=true, is-focused=false, is-focusable=false, is-layout-requested=false, is-selected=false, layout-params=android.widget.LinearLayout$LayoutParams@YYYYYY, tag=null, root-is-layout-requested=false, has-input-connection=false, x=220.0, y=96.0, text=INVALID TYPING, input-type=0, ime-target=false, has-links=false}\n\n\tat dalvik.system.VMStack.getThreadStackTrace(Native Method)\n\tat java.lang.Thread.getStackTrace(Thread.java:1841)\n\tat androidx.test.espresso.base.AssertionErrorHandler.handleSafely(AssertionErrorHandler.java:35)\n\tat androidx.test.espresso.base.AssertionErrorHandler.handleSafely(AssertionErrorHandler.java:26)\n\tat androidx.test.espresso.base.DefaultFailureHandler$TypedFailureHandler.handle(DefaultFailureHandler.java:167)\n\tat androidx.test.espresso.base.DefaultFailureHandler.handle(DefaultFailureHandler.java:128)\n\tat androidx.test.espresso.ViewInteraction.waitForAndHandleInteractionResults(ViewInteraction.java:388)\n\tat androidx.test.espresso.ViewInteraction.check(ViewInteraction.java:367)\n\tat com.example.android.testing.espresso.BasicSample.Test12Test.changeText_sameActivity(Test12Test.java:74)\n\t... 33 trimmed\nCaused by: junit.framework.AssertionFailedError: 'an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"' doesn't match the selected view.\nExpected: an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"\n     Got: view.getText() was \"INVALID TYPING\"\nView Details: TextView{id=2130903044, res-name=textToBeChanged, visibility=VISIBLE, width=328, height=59, has-focus=false, has-focusable=false, has-window-focus=true, is-clickable=false, is-enabled=true, is-focused=false, is-focusable=false, is-layout-requested=false, is-selected=false, layout-params=android.widget.LinearLayout$LayoutParams@YYYYYY, tag=null, root-is-layout-requested=false, has-input-connection=false, x=220.0, y=96.0, text=INVALID TYPING, input-type=0, ime-target=false, has-links=false}\n\n\tat androidx.test.espresso.matcher.ViewMatchers.assertThat(ViewMatchers.java:611)\n\tat androidx.test.espresso.assertion.ViewAssertions$MatchesViewAssertion.check(ViewAssertions.java:97)\n\tat androidx.test.espresso.ViewInteraction$SingleExecutionViewAssertion.check(ViewInteraction.java:489)\n\tat androidx.test.espresso.ViewInteraction$2.call(ViewInteraction.java:347)\n\tat androidx.test.espresso.ViewInteraction$2.call(ViewInteraction.java:320)\n\tat java.util.concurrent.FutureTask.run(FutureTask.java:264)\n\tat android.os.Handler.handleCallback(Handler.java:942)\n\tat android.os.Handler.dispatchMessage(Handler.java:99)\n\tat android.os.Looper.loopOnce(Looper.java:201)\n\tat android.os.Looper.loop(Looper.java:288)\n\tat android.app.ActivityThread.main(ActivityThread.java:7872)\n\tat java.lang.reflect.Method.invoke(Native Method)\n\tat com.android.internal.os.RuntimeInit$MethodAndArgsCaller.run(RuntimeInit.java:548)\n\tat com.android.internal.os.ZygoteInit.main(ZygoteInit.java:936)",
						},
					},
					SystemOut: "<Base System Output>",
				},
			},
		},
		{
			TestSuites: []junit.TestSuite{
				{
					Tests:   1,
					Errors:  1,
					Time:    "6.004",
					Package: "com.example.android.testing.espresso.BasicSample",
					TestCases: []junit.TestCase{
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
							Status:    "error",
							Error:     "androidx.test.espresso.base.AssertionErrorHandler$AssertionFailedWithCauseError: 'an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"' doesn't match the selected view.\nExpected: an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"\n     Got: view.getText() was \"INVALID TYPING\"\nView Details: TextView{id=2130903044, res-name=textToBeChanged, visibility=VISIBLE, width=328, height=59, has-focus=false, has-focusable=false, has-window-focus=true, is-clickable=false, is-enabled=true, is-focused=false, is-focusable=false, is-layout-requested=false, is-selected=false, layout-params=android.widget.LinearLayout$LayoutParams@YYYYYY, tag=null, root-is-layout-requested=false, has-input-connection=false, x=220.0, y=96.0, text=INVALID TYPING, input-type=0, ime-target=false, has-links=false}\n\n\tat dalvik.system.VMStack.getThreadStackTrace(Native Method)\n\tat java.lang.Thread.getStackTrace(Thread.java:1841)\n\tat androidx.test.espresso.base.AssertionErrorHandler.handleSafely(AssertionErrorHandler.java:35)\n\tat androidx.test.espresso.base.AssertionErrorHandler.handleSafely(AssertionErrorHandler.java:26)\n\tat androidx.test.espresso.base.DefaultFailureHandler$TypedFailureHandler.handle(DefaultFailureHandler.java:167)\n\tat androidx.test.espresso.base.DefaultFailureHandler.handle(DefaultFailureHandler.java:128)\n\tat androidx.test.espresso.ViewInteraction.waitForAndHandleInteractionResults(ViewInteraction.java:388)\n\tat androidx.test.espresso.ViewInteraction.check(ViewInteraction.java:367)\n\tat com.example.android.testing.espresso.BasicSample.Test12Test.changeText_sameActivity(Test12Test.java:74)\n\t... 33 trimmed\nCaused by: junit.framework.AssertionFailedError: 'an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"' doesn't match the selected view.\nExpected: an instance of android.widget.TextView and view.getText() with or without transformation to match: is \"Espresso\"\n     Got: view.getText() was \"INVALID TYPING\"\nView Details: TextView{id=2130903044, res-name=textToBeChanged, visibility=VISIBLE, width=328, height=59, has-focus=false, has-focusable=false, has-window-focus=true, is-clickable=false, is-enabled=true, is-focused=false, is-focusable=false, is-layout-requested=false, is-selected=false, layout-params=android.widget.LinearLayout$LayoutParams@YYYYYY, tag=null, root-is-layout-requested=false, has-input-connection=false, x=220.0, y=96.0, text=INVALID TYPING, input-type=0, ime-target=false, has-links=false}\n\n\tat androidx.test.espresso.matcher.ViewMatchers.assertThat(ViewMatchers.java:611)\n\tat androidx.test.espresso.assertion.ViewAssertions$MatchesViewAssertion.check(ViewAssertions.java:97)\n\tat androidx.test.espresso.ViewInteraction$SingleExecutionViewAssertion.check(ViewInteraction.java:489)\n\tat androidx.test.espresso.ViewInteraction$2.call(ViewInteraction.java:347)\n\tat androidx.test.espresso.ViewInteraction$2.call(ViewInteraction.java:320)\n\tat java.util.concurrent.FutureTask.run(FutureTask.java:264)\n\tat android.os.Handler.handleCallback(Handler.java:942)\n\tat android.os.Handler.dispatchMessage(Handler.java:99)\n\tat android.os.Looper.loopOnce(Looper.java:201)\n\tat android.os.Looper.loop(Looper.java:288)\n\tat android.app.ActivityThread.main(ActivityThread.java:7872)\n\tat java.lang.reflect.Method.invoke(Native Method)\n\tat com.android.internal.os.RuntimeInit$MethodAndArgsCaller.run(RuntimeInit.java:548)\n\tat com.android.internal.os.ZygoteInit.main(ZygoteInit.java:936)",
						},
					},
					SystemOut: "<Base System Output>",
				},
			},
		},
		{
			TestSuites: []junit.TestSuite{
				{
					Tests:   1,
					Errors:  0,
					Time:    "5.535",
					Package: "com.example.android.testing.espresso.BasicSample",
					TestCases: []junit.TestCase{
						{
							Name:      "changeText_sameActivity",
							ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
							Status:    "success",
						},
					},
					SystemOut: "<Base System Output>",
				},
			},
		},
	}
	want := junit.TestSuites{
		TestSuites: []junit.TestSuite{
			{
				Tests:   24,
				Errors:  0,
				Time:    "47.917",
				Package: "com.example.android.testing.espresso.BasicSample",
				TestCases: []junit.TestCase{
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test10Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test10Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test11Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test11Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test12Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test1Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test1Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test2Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test2Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test3Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test3Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test4Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test4Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test5Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test5Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test6Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test6Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test7Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test7Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test8Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test8Test",
						Status:    "success",
					},
					{
						Name:      "changeText_newActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test9Test",
						Status:    "success",
					},
					{
						Name:      "changeText_sameActivity",
						ClassName: "com.example.android.testing.espresso.BasicSample.Test9Test",
						Status:    "success",
					},
				},
				SystemOut: "<Base System Output>",
			},
		},
	}

	got := reduceJunitFiles(input)
	if !cmp.Equal(want, got) {
		t.Error(cmp.Diff(want, got))
	}
}
