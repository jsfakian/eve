// Copyright (c) 2023 Zededa, Inc.
// SPDX-License-Identifier: Apache-2.0
package usbmanager

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/lf-edge/eve/pkg/pillar/types"
)

type testingEvent int

const (
	ioBundleTestEvent testingEvent = iota
	usbTestEvent
	vmTestEvent
)

type testEventTableGenerator struct {
	testEventTable [][]testingEvent
}

func (testEvent testingEvent) String() string {
	if testEvent == ioBundleTestEvent {
		return "IOBundle Event"
	} else if testEvent == usbTestEvent {
		return "USB Event"
	} else if testEvent == vmTestEvent {
		return "VM Event"
	}

	return ""
}

func (tetg *testEventTableGenerator) generate(k int, testEvents []testingEvent) {
	// Heap's algorithm
	if k == 1 {
		testEventsCopy := make([]testingEvent, len(testEvents))
		copy(testEventsCopy, testEvents)

		tetg.testEventTable = append(tetg.testEventTable, testEventsCopy)
		return
	}

	tetg.generate(k-1, testEvents)

	for i := 0; i < k-1; i++ {
		var swapIndex int
		if k%2 == 0 {
			swapIndex = i
		} else {
			swapIndex = 0
		}
		t := testEvents[swapIndex]
		testEvents[swapIndex] = testEvents[k-1]
		testEvents[k-1] = t

		tetg.generate(k-1, testEvents)
	}
}

func testEventTable() [][]testingEvent {
	testEvents := []testingEvent{ioBundleTestEvent, usbTestEvent, vmTestEvent}

	var tetg testEventTableGenerator

	tetg.generate(len(testEvents), testEvents)

	return tetg.testEventTable
}

func TestAddNonRuleIOBundle(t *testing.T) {
	uc := newTestUsbmanagerController()
	ioBundle := types.IoBundle{
		Type:         0,
		Phylabel:     "Test",
		Logicallabel: "Test",
	}
	uc.addIOBundle(ioBundle)
	uc.removeIOBundle(ioBundle)

	uc.usbpassthroughs.ioBundles[ioBundle.Phylabel] = &ioBundle

	vm := virtualmachine{
		qmpSocketPath: "",
		adapters:      []string{"Test"},
	}

	uc.addVirtualmachine(vm)
	uc.removeVirtualmachine(vm)
}

func TestRemovingVm(t *testing.T) {
	usbEventBusnum := uint16(1)
	usbEventDevnum := uint16(2)
	usbEventPortnum := "3.1"
	ioBundleUsbAddr := fmt.Sprintf("%d:%s", usbEventBusnum, usbEventPortnum)
	ioBundlePciLong := "00:02.0"
	ioBundleLabel := "TOUCH"
	vmAdapter := ioBundleLabel
	usbEventPCIAddress := ioBundlePciLong
	qmpSocketPath := "/vm/qemu.sock"

	ioBundle, ud, vm := newTestVirtualPassthroughEnv(ioBundleLabel, ioBundleUsbAddr, ioBundlePciLong,
		usbEventBusnum, usbEventDevnum, usbEventPortnum, usbEventPCIAddress,
		qmpSocketPath, vmAdapter)

	uc := newTestUsbmanagerController()
	uc.connectUSBDeviceToQemu = func(up usbpassthrough) {
		t.Logf("connecting usbpassthrough: %+v", up.String())
	}
	uc.disconnectUSBDeviceFromQemu = func(up usbpassthrough) {
		t.Logf("disconnecting usbpassthrough: %+v", up.String())
	}

	uc.addIOBundle(ioBundle)
	uc.addUSBDevice(ud)
	uc.addVirtualmachine(vm)
	if len(uc.usbpassthroughs.usbpassthroughs) != 1 {
		t.Fatalf("invalid amount of usbpassthroughs registered")
	}
	if len(uc.usbpassthroughs.vms) != 1 || len(uc.usbpassthroughs.vmsByIoBundlePhyLabel) != 1 {
		t.Fatalf("invalid amount of vms registered")
	}
	uc.removeVirtualmachine(vm)
	if len(uc.usbpassthroughs.usbpassthroughs) != 0 {
		t.Fatalf("invalid amount of usbpassthroughs registered")
	}
	if len(uc.usbpassthroughs.vms) != 0 || len(uc.usbpassthroughs.vmsByIoBundlePhyLabel) != 0 {
		t.Fatalf("invalid amount of vms registered")
	}
}

func TestNoConnectWrongPCIUSBDevicesToQemu(t *testing.T) {
	usbEventBusnum := uint16(1)
	usbEventDevnum := uint16(2)
	usbEventPortnum := "3.1"
	ioBundleUsbAddr := fmt.Sprintf("%d:%s", usbEventBusnum, usbEventPortnum)
	ioBundlePciLong := "00:02.0"
	ioBundleLabel := "TOUCH"
	vmAdapter := ioBundleLabel
	usbEventPCIAddress := ""
	qmpSocketPath := "/vm/qemu.sock"

	ioBundle, usbdevice, vm := newTestVirtualPassthroughEnv(ioBundleLabel, ioBundleUsbAddr, ioBundlePciLong,
		usbEventBusnum, usbEventDevnum, usbEventPortnum, usbEventPCIAddress,
		qmpSocketPath, vmAdapter)

	tet := testEventTable()
	countUSBConnections := testRunConnectingUsbDevicesOrderCombinations(tet, qmpSocketPath, ioBundle, usbdevice, vm)

	if countUSBConnections != 0 {
		t.Fatalf("expected 0 connection attempts to qemu, but got %d", countUSBConnections)
	}
}

func TestNoConnectUSBDevicesToQemu(t *testing.T) {
	usbEventBusnum := uint16(1)
	usbEventDevnum := uint16(2)
	usbEventPortnum := "3.1"
	ioBundleUsbAddr := fmt.Sprintf("%d:%s-1", usbEventBusnum, usbEventPortnum) // usb port different from usb device
	ioBundlePciLong := "00:02.0"
	ioBundleLabel := "TOUCH"
	vmAdapter := ioBundleLabel
	usbEventPCIAddress := ioBundlePciLong
	qmpSocketPath := "/vm/qemu.sock"

	ioBundle, usbdevice, vm := newTestVirtualPassthroughEnv(ioBundleLabel, ioBundleUsbAddr, ioBundlePciLong,
		usbEventBusnum, usbEventDevnum, usbEventPortnum, usbEventPCIAddress,
		qmpSocketPath, vmAdapter)

	tet := testEventTable()
	countUSBConnections := testRunConnectingUsbDevicesOrderCombinations(tet, qmpSocketPath, ioBundle, usbdevice, vm)

	if countUSBConnections != 0 {
		t.Fatalf("expected 0 connection attempts to qemu, but got %d", countUSBConnections)
	}
}

func TestReconnectUSBDevicesToQemu(t *testing.T) {
	usbEventBusnum := uint16(1)
	usbEventDevnum := uint16(2)
	usbEventPortnum := "3.1"
	ioBundleUsbAddr := fmt.Sprintf("%d:%s", usbEventBusnum, usbEventPortnum)
	ioBundlePciLong := "00:02.0"
	ioBundleLabel := "TOUCH"
	vmAdapter := ioBundleLabel
	usbEventPCIAddress := ioBundlePciLong
	qmpSocketPath := "/vm/qemu.sock"

	ioBundle, ud, vm := newTestVirtualPassthroughEnv(ioBundleLabel, ioBundleUsbAddr, ioBundlePciLong,
		usbEventBusnum, usbEventDevnum, usbEventPortnum, usbEventPCIAddress,
		qmpSocketPath, vmAdapter)

	uc := newTestUsbmanagerController()
	var countCurrentUSBPassthroughs atomic.Int32
	countCurrentUSBPassthroughs.Store(0)
	uc.connectUSBDeviceToQemu = func(up usbpassthrough) {
		countCurrentUSBPassthroughs.Add(1)
	}
	uc.disconnectUSBDeviceFromQemu = func(up usbpassthrough) {
		countCurrentUSBPassthroughs.Add(-1)
	}

	uc.addIOBundle(ioBundle)
	uc.addUSBDevice(ud)
	uc.addUSBDevice(ud)
	uc.addVirtualmachine(vm)
	uc.addUSBDevice(ud)
	if countCurrentUSBPassthroughs.Load() != 1 {
		t.Fatalf("expected current usb passthrough count to be 1, but got %d", countCurrentUSBPassthroughs.Load())
	}
	uc.removeUSBDevice(ud)
	uc.removeUSBDevice(ud)
	if countCurrentUSBPassthroughs.Load() != 0 {
		t.Fatalf("expected current usb passthrough count to be 0, but got %d", countCurrentUSBPassthroughs.Load())
	}

	uc.addUSBDevice(ud)
	if countCurrentUSBPassthroughs.Load() != 1 {
		t.Fatalf("expected current usb passthrough count to be 1, but got %d", countCurrentUSBPassthroughs.Load())
	}
	uc.addUSBDevice(ud)

	if countCurrentUSBPassthroughs.Load() != 1 {
		t.Fatalf("expected current usb passthrough count to be 1, but got %d", countCurrentUSBPassthroughs.Load())
	}
}

func TestConnectUSBDevicesToQemu(t *testing.T) {
	usbEventBusnum := uint16(1)
	usbEventDevnum := uint16(2)
	usbEventPortnum := "3.1"
	ioBundleUsbAddr := fmt.Sprintf("%d:%s", usbEventBusnum, usbEventPortnum)
	ioBundlePciLong := "00:02.0"
	ioBundleLabel := "TOUCH"
	vmAdapter := ioBundleLabel
	usbEventPCIAddress := ioBundlePciLong
	qmpSocketPath := "/vm/qemu.sock"

	ioBundle, usbdevice, vm := newTestVirtualPassthroughEnv(ioBundleLabel, ioBundleUsbAddr, ioBundlePciLong,
		usbEventBusnum, usbEventDevnum, usbEventPortnum, usbEventPCIAddress,
		qmpSocketPath, vmAdapter)

	tet := testEventTable()
	countUSBConnections := testRunConnectingUsbDevicesOrderCombinations(tet, qmpSocketPath, ioBundle, usbdevice, vm)

	if len(tet) != countUSBConnections {
		t.Fatalf("expected %d connection attempts to qemu, but got %d", len(tet), countUSBConnections)
	}
}

func TestIOBundleEmpty(t *testing.T) {
	bundle := types.IoBundle{}

	pr := ioBundle2PassthroughRule(bundle)

	if pr != nil {
		t.Fatalf("expected nil rule but got %+v", pr)
	}
}

func TestIOBundlePCIForbidRule(t *testing.T) {
	bundle := types.IoBundle{PciLong: "00:14.0"}

	pr := ioBundle2PassthroughRule(bundle)

	_, ok := pr.(*pciPassthroughForbidRule)
	if !ok {
		t.Fatalf("expected pciPassthroughForbidRule type but got %T %+v", pr, pr)
	}
}

func TestIOBundlePCIAndUSBProduct(t *testing.T) {
	bundle := types.IoBundle{
		PciLong:    "0:0",
		UsbProduct: "1:1",
	}

	pr := ioBundle2PassthroughRule(bundle)

	hasPCIRule := false
	hasUSBProductRule := false

	cpr := pr.(*compositionPassthroughRule)
	for _, rule := range cpr.rules {
		switch rule.(type) {
		case *pciPassthroughRule:
			hasPCIRule = true
		case *usbDevicePassthroughRule:
			hasUSBProductRule = true
		}
	}

	if !hasPCIRule {
		t.Fatal("not pciPassthroughRule")
	}
	if !hasUSBProductRule {
		t.Fatal("not usbDevicePassthroughRule")
	}

	ud := usbdevice{
		usbControllerPCIAddress: "2:2",
	}

	action := pr.evaluate(ud)
	if action != passthroughNo {
		t.Fatalf("passthrough action should be passthroughNo, but got %v", action)
	}

	ud.vendorID = 1
	ud.productID = 1
	action = pr.evaluate(ud)
	if action != passthroughNo {
		t.Fatalf("passthrough action should be passthroughNo, but got %v", action)
	}
	ud.usbControllerPCIAddress = "0:0"
	action = pr.evaluate(ud)
	if action != passthroughDo {
		t.Fatalf("passthrough action should be passthroughDo, but got %v", action)
	}
}

func TestIOBundlePCIAndUSBAddress(t *testing.T) {
	bundle := types.IoBundle{
		PciLong: "0:0",
		UsbAddr: "1:1",
	}

	pr := ioBundle2PassthroughRule(bundle)

	ud := usbdevice{
		usbControllerPCIAddress: "2:2",
	}

	action := pr.evaluate(ud)
	if action != passthroughNo {
		t.Fatalf("passthrough action should be passthroughNo, but got %v", action)
	}

	ud.busnum = 1
	ud.portnum = "1"
	action = pr.evaluate(ud)
	if action != passthroughNo {
		t.Fatalf("passthrough action should be passthroughNo, but got %v", action)
	}
	ud.usbControllerPCIAddress = "0:0"
	action = pr.evaluate(ud)
	if action != passthroughDo {
		t.Fatalf("passthrough action should be passthroughDo, but got %v", action)
	}
}

func TestIOBundleUSBProductAndUSBAddress(t *testing.T) {
	bundle := types.IoBundle{
		UsbAddr:    "1:1",
		UsbProduct: "2:2",
	}

	pr := ioBundle2PassthroughRule(bundle)

	ud := usbdevice{}

	action := pr.evaluate(ud)
	if action != passthroughNo {
		t.Fatalf("passthrough action should be passthroughNo, but got %v", action)
	}

	for _, test := range []struct {
		beforeFunc     func()
		expectedAction passthroughAction
	}{
		{
			beforeFunc:     func() { ud.busnum = 1 },
			expectedAction: passthroughNo,
		},
		{
			beforeFunc:     func() { ud.portnum = "1" },
			expectedAction: passthroughNo,
		},
		{
			beforeFunc:     func() { ud.vendorID = 2 },
			expectedAction: passthroughNo,
		},
		{
			beforeFunc:     func() { ud.productID = 2 },
			expectedAction: passthroughDo,
		},
		{
			beforeFunc: func() {
				bundle.PciLong = "3:3" // passthrough is now tied to this pci controller
				pr = ioBundle2PassthroughRule(bundle)

				ud.usbControllerPCIAddress = "4:4"
			},
			expectedAction: passthroughNo,
		},
		{
			beforeFunc:     func() { ud.usbControllerPCIAddress = "3:3" },
			expectedAction: passthroughDo,
		},
	} {
		test.beforeFunc()
		action := pr.evaluate(ud)
		if action != test.expectedAction {
			t.Fatalf("passthrough action should be %v, but got %v; ud: %+v", test.expectedAction, action, ud)
		}
	}
}

func testRunConnectingUsbDevicesOrderCombinations(tet [][]testingEvent, expectedQmpSocketPath string, ioBundle types.IoBundle, ud usbdevice, vm virtualmachine) int {
	countUSBConnections := 0
	for _, testEvents := range tet {
		uc := newTestUsbmanagerController()

		uc.connectUSBDeviceToQemu = func(up usbpassthrough) {
			if up.vm.qmpSocketPath != expectedQmpSocketPath {
				err := fmt.Errorf("vm connecting to should have qmp path %s, but has %s", expectedQmpSocketPath, up.vm.qmpSocketPath)
				panic(err)
			}
			countUSBConnections++
		}

		for _, testEvent := range testEvents {
			if testEvent == ioBundleTestEvent {
				uc.addIOBundle(ioBundle)
			} else if testEvent == usbTestEvent {
				uc.addUSBDevice(ud)
			} else if testEvent == vmTestEvent {
				uc.addVirtualmachine(vm)
			}
		}

	}
	return countUSBConnections
}

func newTestVirtualPassthroughEnv(ioBundleLabel, ioBundleUsbAddr, ioBundlePciLong string,
	usbEventBusnum, usbEventDevnum uint16, usbEventPortnum string, usbEventPCIAddress,
	qmpSocketPath, vmAdapter string) (types.IoBundle, usbdevice, virtualmachine) {

	ioBundle := types.IoBundle{Phylabel: ioBundleLabel, UsbAddr: ioBundleUsbAddr, PciLong: ioBundlePciLong}

	ud := usbdevice{
		busnum:                  usbEventBusnum,
		devnum:                  usbEventDevnum,
		portnum:                 usbEventPortnum,
		vendorID:                05,
		productID:               06,
		usbControllerPCIAddress: usbEventPCIAddress,
	}
	vm := virtualmachine{
		qmpSocketPath: qmpSocketPath,
		adapters:      []string{vmAdapter},
	}

	return ioBundle, ud, vm
}

func newTestUsbmanagerController() *usbmanagerController {
	uc := usbmanagerController{}
	uc.init()

	return &uc
}

func FuzzUSBManagerController(f *testing.F) {
	f.Fuzz(func(t *testing.T,
		phyLabel1 string,
		pciLong1 string,
		usbaddr1 string,
		usbproduct1 string,
		assigngrp1 string,

		phyLabel2 string,
		pciLong2 string,
		usbaddr2 string,
		usbproduct2 string,
		assigngrp2 string,

		phyLabel3 string,
		pciLong3 string,
		usbaddr3 string,
		usbproduct3 string,
		assigngrp3 string,

		phyLabel4 string,
		pciLong4 string,
		usbaddr4 string,
		usbproduct4 string,
		assigngrp4 string,

		phyLabel5 string,
		pciLong5 string,
		usbaddr5 string,
		usbproduct5 string,
		assigngrp5 string,

		delBundle1 uint,
		delBundle1Pos uint,

		delBundle2 uint,
		delBundle2Pos uint,

		delBundle3 uint,
		delBundle3Pos uint,

		addVm1Name string,
		addVm1Adapter1 uint,
		addVm1Adapter2 uint,
		addVm1Adapter3 uint,
		addVm1Pos uint,

		addVm2Name string,
		addVm2Adapter1 uint,
		addVm2Adapter2 uint,
		addVm2Adapter3 uint,
		addVm2Pos uint,

		delVm1Pos uint,
		delVm2Pos uint,

		addUSB1Dev uint,
		addUSB1Pos uint,

		addUSB2Dev uint,
		addUSB2Pos uint,

	) {

		ioBundle1 := types.IoBundle{
			Phylabel:        phyLabel1,
			AssignmentGroup: assigngrp1,
			PciLong:         pciLong1,
			UsbAddr:         usbaddr1,
			UsbProduct:      usbproduct1,
		}

		ioBundle2 := types.IoBundle{
			Phylabel:        phyLabel2,
			AssignmentGroup: assigngrp2,
			PciLong:         pciLong2,
			UsbAddr:         usbaddr2,
			UsbProduct:      usbproduct2,
		}

		ioBundle3 := types.IoBundle{
			Phylabel:        phyLabel3,
			AssignmentGroup: assigngrp3,
			PciLong:         pciLong3,
			UsbAddr:         usbaddr3,
			UsbProduct:      usbproduct3,
		}

		ioBundle4 := types.IoBundle{
			Phylabel:        phyLabel4,
			AssignmentGroup: assigngrp4,
			PciLong:         pciLong4,
			UsbAddr:         usbaddr4,
			UsbProduct:      usbproduct4,
		}

		ioBundle5 := types.IoBundle{
			Phylabel:        phyLabel5,
			AssignmentGroup: assigngrp5,
			PciLong:         pciLong5,
			UsbAddr:         usbaddr5,
			UsbProduct:      usbproduct5,
		}

		ioBundlesArray := []*types.IoBundle{&ioBundle1, &ioBundle2, &ioBundle3, &ioBundle4, &ioBundle5}
		ioBundlesArrayLen := uint(len(ioBundlesArray))

		for i := range ioBundlesArray {
			_, size := utf8.DecodeLastRuneInString(ioBundlesArray[i].AssignmentGroup)
			// set the parentassigngrp to the assigngrp without the last character
			// this way it is guaranteed that ioBundles with the same assigngrp
			// have the same parentassigngrp
			parentassigngrp := ioBundlesArray[i].AssignmentGroup[:len(ioBundlesArray[i].AssignmentGroup)-size]

			ioBundlesArray[i].ParentAssignmentGroup = parentassigngrp
		}

		addUSBCmd := []struct {
			ud  usbdevice
			pos uint
		}{
			{
				ud:  createTestUSBDeviceFromIOBundle(ioBundlesArray[addUSB1Dev%ioBundlesArrayLen]),
				pos: addUSB1Pos % ioBundlesArrayLen,
			},
			{
				ud:  createTestUSBDeviceFromIOBundle(ioBundlesArray[addUSB2Dev%ioBundlesArrayLen]),
				pos: addUSB2Pos % ioBundlesArrayLen,
			},
		}

		addVMCmd := []struct {
			vm  virtualmachine
			pos uint
		}{
			{
				vm:  createTestVM(addVm1Name, ioBundlesArray, addVm1Adapter1, addVm1Adapter2, addVm1Adapter3),
				pos: addVm1Pos % ioBundlesArrayLen,
			},
			{
				vm:  createTestVM(addVm2Name, ioBundlesArray, addVm2Adapter1, addVm2Adapter2, addVm2Adapter3),
				pos: addVm2Pos % ioBundlesArrayLen,
			},
		}
		if addVm1Name == addVm2Name {
			t.Log("vm1 and vm2 have the same name")
		}

		delBundleCmd := []struct {
			index uint
			pos   uint
		}{
			{delBundle1, delBundle1Pos % ioBundlesArrayLen},
			{delBundle2, delBundle2Pos % ioBundlesArrayLen},
			{delBundle3, delBundle3Pos % ioBundlesArrayLen},
		}

		for i := range delBundleCmd {
			delBundleCmd[i].index = delBundleCmd[i].index % ioBundlesArrayLen
			delBundleCmd[i].pos = delBundleCmd[i].pos % ioBundlesArrayLen
		}

		umc := usbmanagerController{}
		umc.init()
		umc.connectUSBDeviceToQemu = func(up usbpassthrough) {
			t.Logf("connect usbdevice: %+v", up)
		}
		umc.disconnectUSBDeviceFromQemu = func(up usbpassthrough) {
			t.Logf("disconnect usbdevice: %+v", up)
		}
		for pos, ioBundle := range ioBundlesArray {
			for _, dbc := range delBundleCmd {
				if dbc.pos == uint(pos) {
					removeIOBundle := ioBundlesArray[dbc.index]
					if removeIOBundle != nil {
						t.Logf("removing ioBundle label %s usbaddr: %s usbproduct: %s pcilong: %s",
							ioBundle.Phylabel, removeIOBundle.UsbAddr, removeIOBundle.UsbProduct, removeIOBundle.PciLong)
						umc.removeIOBundle(*removeIOBundle)
					}
				}
			}

			for _, avc := range addVMCmd {
				if avc.pos == uint(pos) {
					t.Logf("adding virtualmachine with adapters %+v", avc.vm.adapters)
					umc.addVirtualmachine(avc.vm)
				}
			}

			for _, udc := range addUSBCmd {
				if int(udc.pos) == pos {
					t.Logf("adding device %+v", udc.ud)
					umc.addUSBDevice(udc.ud)
				}
			}

			if delVm1Pos == uint(pos) {
				t.Logf("removing virtualmachine with adapters %+v", addVMCmd[0].vm.adapters)
				umc.removeVirtualmachine(addVMCmd[0].vm)
			}
			if delVm2Pos == uint(pos) {
				t.Logf("removing virtualmachine with adapters %+v", addVMCmd[1].vm.adapters)
				umc.removeVirtualmachine(addVMCmd[1].vm)
			}

			t.Logf("adding ioBundle label %s usbaddr: %s usbproduct: %s pcilong: %s",
				ioBundle.Phylabel, ioBundle.UsbAddr, ioBundle.UsbProduct, ioBundle.PciLong)
			umc.addIOBundle(*ioBundle)
		}
	})
}

func createTestUSBDeviceFromIOBundle(ioBundle *types.IoBundle) usbdevice {
	var ud usbdevice

	ud.usbControllerPCIAddress = ioBundle.PciLong
	usbParts := strings.SplitN(ioBundle.UsbAddr, ":", 2)

	busnum, _ := strconv.ParseUint(usbParts[0], 10, 16)

	ud.busnum = uint16(busnum)
	if len(usbParts) == 2 {
		ud.portnum = usbParts[1]
	}

	usbParts = strings.SplitN(ioBundle.UsbProduct, ":", 2)

	vendorID, _ := strconv.ParseUint(usbParts[0], 16, 32)
	ud.vendorID = uint32(vendorID)

	if len(usbParts) == 2 {
		productID, _ := strconv.ParseUint(usbParts[1], 16, 32)
		ud.productID = uint32(productID)
	}

	return ud
}

func createTestVM(vmName string, ioBundlesArray []*types.IoBundle, vmAdapter1 uint, vmAdapter2 uint, vmAdapter3 uint) virtualmachine {
	vm := virtualmachine{
		qmpSocketPath: vmName,
		adapters:      []string{},
	}
	for _, adapterIndex := range []int{int(vmAdapter1), int(vmAdapter2), int(vmAdapter3)} {
		pos := adapterIndex % len(ioBundlesArray)

		if adapterIndex > 0 {
			vm.addAdapter(ioBundlesArray[pos].Phylabel)
		}
	}

	return vm
}
